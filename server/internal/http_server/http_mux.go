package http_server

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/3rdparty/grpc-gateway/httprule"
	http2 "github.com/zhangel/go-framework/http"
	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/memory_registry"
	"github.com/zhangel/go-framework/registry"
	"github.com/zhangel/go-framework/server/internal/grpc_server"
	"github.com/zhangel/go-framework/server/internal/option"
)

type httpInvokeCtxKey int

var HttpInvokeCtx httpInvokeCtxKey

type peerAddr struct {
	network string
	addr    string
}

func (s peerAddr) Network() string {
	return s.network
}

func (s peerAddr) String() string {
	return s.addr
}

type ServerMux struct {
	srv            *grpc.Server
	opts           *option.Options
	marshaler      *runtime.JSONPb
	rawPostMatcher *http2.PathMatcher
	ruleMatcher    *http2.PathMatcher
	httpRules      []*registry.HttpRule
}

type _HeaderMatcher struct {
	serviceOpt *_ServiceOpt
}

func (s *_HeaderMatcher) HeaderMap() []map[string]string {
	if s.serviceOpt == nil {
		return nil
	}

	return s.serviceOpt.headerMap
}

func (s *_HeaderMatcher) HeaderRegexMap() []map[string]*regexp.Regexp {
	if s.serviceOpt == nil {
		return nil
	}

	return s.serviceOpt.headerRegexMap
}

func (s *_HeaderMatcher) Priority() int {
	if len(s.HeaderMap()) > 0 || len(s.HeaderRegexMap()) > 0 {
		return 1
	} else {
		return 0
	}
}

type _RawDesc struct {
	_HeaderMatcher
	method InvokerWrapper
}

type _HttpRuleDesc struct {
	_HeaderMatcher
	method           InvokerWrapper
	pattern          runtime.Pattern
	filter           *utilities.DoubleArray
	bodyField        string
	respField        string
	parameterHandler map[string]option.ParameterHandler
}

type _ServiceOpt struct {
	headerMap            []map[string]string
	headerRegexStringMap []map[string]string
	headerRegexMap       []map[string]*regexp.Regexp
}

func NewServerMux(srv *grpc.Server, grpcServiceDesc grpc_server.ServiceDescList, opts *option.Options) *ServerMux {
	var marshaler *runtime.JSONPb
	if opts.GrpcGatewayCompatible {
		marshaler = &runtime.JSONPb{OrigName: true}
	} else {
		marshaler = &runtime.JSONPb{
			OrigName:     opts.OrigName,
			EnumsAsInts:  opts.EnumAsInts,
			EmitDefaults: opts.EmitDefaults,
		}
	}

	mux := &ServerMux{
		srv:            srv,
		opts:           opts,
		marshaler:      marshaler,
		rawPostMatcher: http2.NewPathMatcher(),
		ruleMatcher:    http2.NewPathMatcher(),
	}

	if err := mux.registerMethods(grpcServiceDesc); err != nil {
		log.Fatal(err)
	}

	return mux
}

func (s *ServerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.Internal, "%s", err))
		}
	}()

	if override := r.Header.Get("X-HTTP-Method-Override"); override != "" && r.Method == "POST" {
		r.Method = strings.ToUpper(override)
		contentType := r.Header.Get("Content-Type")
		switch contentType {
		case "application/x-www-form-urlencoded":
			if err := r.ParseForm(); err != nil {
				log.Error(err)
				httpErrorHandler(0, w, s.marshaler, status.Error(codes.InvalidArgument, err.Error()))
				return
			}
		case "application/json":
			break
		default:
			log.Warnf("Use X-HTTP-Method-Override with invalid content-type[%s]", contentType)
		}
	}

	req, err := s.makeRequest(r)
	if err != nil {
		log.Error(err)
		httpErrorHandler(0, w, s.marshaler, err)
		return
	}

	ctx := r.Context()
	if r.TLS != nil {
		ctx = peer.NewContext(ctx, &peer.Peer{
			Addr:     peerAddr{network: "https", addr: r.RemoteAddr},
			AuthInfo: credentials.TLSInfo{State: *r.TLS},
		})

	}

	req(requestCtxWithHeader(ctx, r, s.opts.GrpcGatewayCompatible), w)
}

func (s *ServerMux) serviceOptions(alias []string) (*_ServiceOpt, error) {
	if len(alias) == 0 {
		return nil, nil
	}

	opt := &_ServiceOpt{}
	for _, a := range alias {
		opt.headerMap = append(opt.headerMap, map[string]string{"X-F-Service": a})
	}

	return opt, nil
}

func (s *ServerMux) registerMethods(grpcServiceDesc grpc_server.ServiceDescList) error {
	if len(s.srv.GetServiceInfo()) == 0 {
		return nil
	}

	headerAlias := map[string][]string{}
	parameterHandler := map[string]map[string]option.ParameterHandler{}

	for _, sd := range grpcServiceDesc {
		for _, alias := range sd.RegisterOption.Alias {
			headerAlias[sd.ServiceInfo.ServiceName] = append(headerAlias[sd.ServiceInfo.ServiceName], alias)
		}
		if _, ok := headerAlias[sd.ServiceInfo.ServiceName]; !ok {
			headerAlias[sd.ServiceInfo.ServiceName] = append(headerAlias[sd.ServiceInfo.ServiceName], sd.ServiceInfo.ServiceName)
		}

		if len(sd.RegisterOption.HttpParameterHandler) != 0 {
			m, ok := parameterHandler[sd.ServiceInfo.ServiceName]
			if !ok {
				m = map[string]option.ParameterHandler{}
				parameterHandler[sd.ServiceInfo.ServiceName] = m
			}

			for k, v := range sd.RegisterOption.HttpParameterHandler {
				m[k] = v
			}
		}
	}

	for serviceFullName, serviceInfo := range s.srv.GetServiceInfo() {
		log.Info(strings.Repeat("=", horizontalLineWidth))
		fdName, ok := serviceInfo.Metadata.(string)
		if !ok {
			log.Infof("[ERROR] Get proto file descriptor meta[%s] from service info failed.", serviceFullName)
			continue
		}

		fd, err := s.extractFileDescriptor(proto.FileDescriptor(fdName))
		if err != nil {
			log.Infof("[ERROR] Extract file descriptor for %s failed, err = %v.", serviceFullName, err)
			continue
		}

		for _, serviceDesc := range fd.GetService() {
			serviceName := serviceDesc.GetName()
			if serviceFullName != fd.GetPackage()+"."+serviceName {
				continue
			}

			serviceOpt, err := s.serviceOptions(headerAlias[serviceFullName])
			if err != nil {
				return fmt.Errorf("parse service options failed, err = %v", err)
			}

			var headerMap []map[string]string
			var headerRegexMap []map[string]string

			if serviceOpt != nil {
				headerMap = serviceOpt.headerMap
				headerRegexMap = serviceOpt.headerRegexStringMap
			}

			registryHttpRule := &registry.HttpRule{
				HeaderMap:      headerMap,
				HeaderRegexMap: headerRegexMap,
			}

			log.Infof("Register service: %s", serviceFullName)
			if len(headerMap) != 0 || len(headerRegexMap) != 0 {
				log.Info(strings.Repeat("-", horizontalLineWidth))
				if len(headerMap) != 0 {
					log.Infof("HeaderPattern: %+v", headerMap)
				}
				if len(headerRegexMap) != 0 {
					log.Infof("HeaderRegexPattern: %+v", headerRegexMap)
				}
			}
			log.Info(strings.Repeat("-", horizontalLineWidth))

			for _, methodDesc := range serviceDesc.GetMethod() {
				if methodDesc.GetClientStreaming() {
					continue
				}

				var wrapper InvokerWrapper
				if !methodDesc.GetServerStreaming() {
					unary, err := memory_registry.GlobalRegistry.QueryUnary(serviceFullName, "/"+serviceFullName+"/"+methodDesc.GetName())
					if err != nil {
						log.Warnf("[ERROR] Match method[%s] between descriptor and service-impl failed.", methodDesc.GetName())
						continue
					}

					wrapper = NewUnaryInvoker(unary)
				} else {
					streaming, err := memory_registry.GlobalRegistry.QueryStreaming(serviceFullName, "/"+serviceFullName+"/"+methodDesc.GetName())
					if err != nil {
						log.Warnf("[ERROR] Match method[%s] between descriptor and service-impl failed.", methodDesc.GetName())
						continue
					}

					wrapper = NewServerStreamingInvoker(streaming)
				}

				log.Infof("Register HTTP handlers for %q:", wrapper.Method())
				if err := s.registerMethod(fd.GetPackage(), serviceName, serviceOpt, registryHttpRule, wrapper, methodDesc, parameterHandler[serviceFullName]); err != nil {
					log.Warnf("\t[ERROR]", err)
				}
				log.Info(strings.Repeat("-", horizontalLineWidth))
			}
			s.httpRules = append(s.httpRules, registryHttpRule)
		}
	}
	return nil
}

func (s *ServerMux) extractFileDescriptor(fd_bytes []byte) (*descriptor.FileDescriptorProto, error) {
	r, err := gzip.NewReader(bytes.NewReader(fd_bytes))
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip reader: %v", err)
	}
	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to uncompress descriptor: %v", err)
	}

	fd := new(descriptor.FileDescriptorProto)
	if err := proto.Unmarshal(b, fd); err != nil {
		return nil, fmt.Errorf("malformed FileDescriptorProto: %v", err)
	}

	return fd, nil
}

func (s *ServerMux) registerRawPostApi(packageName, serviceName string, serviceOpt *_ServiceOpt, registryHttpRule *registry.HttpRule, invokerWrapper InvokerWrapper, methodDesc *descriptor.MethodDescriptorProto) error {
	if s.opts.WithPackageName {
		serviceName = packageName + "." + serviceName
	}

	rawPostPath := fmt.Sprintf("/%s/%s", serviceName, methodDesc.GetName())
	matcherPattern, err := http2.RegistryWithGrpcPattern("POST" + rawPostPath)
	if err != nil {
		return fmt.Errorf("convert restful register pattern failed, err = %v", err)
	}

	if err := s.rawPostMatcher.AddPattern("", matcherPattern, &_RawDesc{
		method: invokerWrapper,
		_HeaderMatcher: _HeaderMatcher{
			serviceOpt: serviceOpt,
		},
	}); err != nil {
		return fmt.Errorf("add pattern %s to rawPostMatcher failed, err = %v", matcherPattern, err)
	}

	registryHttpRule.HttpPatterns = append(registryHttpRule.HttpPatterns, matcherPattern)
	log.Info("\t", s.methodPatternString("POST", rawPostPath, "*", ""))
	return nil
}

func (s *ServerMux) registerHttpRule(serviceOpt *_ServiceOpt, registryHttpRule *registry.HttpRule, invokerWrapper InvokerWrapper, httpRule *annotations.HttpRule, parameterHandler map[string]option.ParameterHandler) error {
	var err error
	body := strings.TrimSpace(httpRule.GetBody())
	responseBody := strings.TrimSpace(httpRule.GetResponseBody())

	switch rule := httpRule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, "GET", rule.Get, body, responseBody, parameterHandler)
	case *annotations.HttpRule_Post:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, "POST", rule.Post, body, responseBody, parameterHandler)
	case *annotations.HttpRule_Put:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, "PUT", rule.Put, body, responseBody, parameterHandler)
	case *annotations.HttpRule_Delete:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, "DELETE", rule.Delete, body, responseBody, parameterHandler)
	case *annotations.HttpRule_Patch:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, "PATCH", rule.Patch, body, responseBody, parameterHandler)
	case *annotations.HttpRule_Custom:
		err = s.registerPattern(serviceOpt, registryHttpRule, invokerWrapper, strings.ToUpper(rule.Custom.Kind), rule.Custom.Path, body, responseBody, parameterHandler)
	default:
		err = fmt.Errorf("invalid http rule annotation '%v'", httpRule)
	}
	return err
}

func (s *ServerMux) registerMethod(
	packageName, serviceName string,
	serviceOpt *_ServiceOpt,
	registryHttpRule *registry.HttpRule,
	invokerWrapper InvokerWrapper,
	methodDesc *descriptor.MethodDescriptorProto,
	parameterHandler map[string]option.ParameterHandler,
) error {
	extensionDescs, err := proto.ExtensionDescs(methodDesc.GetOptions())
	if err != nil {
		return err
	}
	extensions, err := proto.GetExtensions(methodDesc.GetOptions(), extensionDescs)
	if err != nil {
		return err
	}

	noHttpRule := true
	for _, extension := range extensions {
		httpRule, ok := extension.(*annotations.HttpRule)
		if !ok {
			continue
		}

		noHttpRule = false
		if err := s.registerHttpRule(serviceOpt, registryHttpRule, invokerWrapper, httpRule, parameterHandler); err != nil {
			return err
		}

		for _, additionalBindings := range httpRule.GetAdditionalBindings() {
			if err := s.registerHttpRule(serviceOpt, registryHttpRule, invokerWrapper, additionalBindings, parameterHandler); err != nil {
				return err
			}
		}
	}

	if noHttpRule {
		if err := s.registerRawPostApi(packageName, serviceName, serviceOpt, registryHttpRule, invokerWrapper, methodDesc); err != nil {
			return err
		}
	}

	return nil
}

func (s *ServerMux) registerPattern(serviceOpt *_ServiceOpt, registryHttpRule *registry.HttpRule, invokerWrapper InvokerWrapper, httpMethod, pathPat, bodyField, respField string, parameterHandler map[string]option.ParameterHandler) (err error) {
	defer func() {
		if err != nil {
			log.Warn("[ERROR]\t", s.methodPatternString(httpMethod, pathPat, bodyField, respField), "| Reason:", err)
		}
	}()

	var seqs [][]string
	if bodyField != "" {
		seqs = append(seqs, strings.Split(bodyField, "."))
	}

	pathCompiler, err := httprule.Parse(pathPat)
	if err != nil {
		return err
	}

	pathTemplate := pathCompiler.Compile()
	for _, p := range pathTemplate.Fields {
		seqs = append(seqs, strings.Split(p, "."))
	}

	pat, err := runtime.NewPattern(1, pathTemplate.OpCodes, pathTemplate.Pool, pathTemplate.Verb)
	if err != nil {
		return err
	}

	for _, segment := range pathTemplate.Fields {
		typ := invokerWrapper.RequestType()
		for _, p := range strings.Split(segment, ".") {
			if typInner, ok := typ.Elem().FieldByName(generator.CamelCase(p)); !ok {
				return fmt.Errorf("no %q field found in %s", segment, invokerWrapper.RequestType().Elem())
			} else {
				typ = typInner.Type
			}
		}
	}

	if strings.Contains(bodyField, ".") {
		return fmt.Errorf("referred field in body must be present at the top-level of the request message type")
	}

	if strings.Contains(respField, ".") {
		return fmt.Errorf("referred field in response-body must be present at the top-level of the request message type")
	}

	if _, ok := invokerWrapper.RequestType().Elem().FieldByName(generator.CamelCase(bodyField)); !ok && bodyField != "" && bodyField != "*" {
		return fmt.Errorf("no %q field found in %s", bodyField, invokerWrapper.RequestType().Elem())
	}

	if _, ok := invokerWrapper.ResponseType().Elem().FieldByName(generator.CamelCase(respField)); !ok && respField != "" && respField != "*" {
		return fmt.Errorf("no %q field found in %s", respField, invokerWrapper.ResponseType().Elem())
	}

	pattern := httpMethod + pathPat
	matcherPattern, err := http2.RegistryWithGrpcPattern(pattern)
	if err != nil {
		return fmt.Errorf("convert restful register pattern failed, err = %v", err)
	}

	if err := s.ruleMatcher.AddPattern("", matcherPattern, &_HttpRuleDesc{
		method:           invokerWrapper,
		pattern:          pat,
		filter:           utilities.NewDoubleArray(seqs),
		bodyField:        bodyField,
		respField:        respField,
		parameterHandler: parameterHandler,
		_HeaderMatcher: _HeaderMatcher{
			serviceOpt: serviceOpt,
		},
	}); err != nil {
		return fmt.Errorf("add pattern %s to ruleMatcher failed, err = %v", matcherPattern, err)
	}

	registryHttpRule.HttpPatterns = append(registryHttpRule.HttpPatterns, matcherPattern)
	log.Info("\t", s.methodPatternString(httpMethod, pathPat, bodyField, respField))
	return nil
}

func (s *ServerMux) makeRequest(r *http.Request) (func(ctx context.Context, w http.ResponseWriter), error) {
	var errs []error
	if request, err := s.makeRawPostRequest(r); err == nil && request != nil {
		return request, nil
	} else if err != nil {
		errs = append(errs, err)
	}

	if request, err := s.makeHttpRuleRequest(r); err == nil && request != nil {
		return request, nil
	} else if err != nil {
		errs = append(errs, err)
	}

	st, _ := status.New(codes.NotFound, codes.NotFound.String()).WithDetails(&errdetails.DebugInfo{Detail: fmt.Sprintf("%+v", errs)})
	return nil, st.Err()
}

func (s *ServerMux) makeRawPostHandler(r *http.Request, m InvokerWrapper) func(ctx context.Context, w http.ResponseWriter) {
	if m == nil {
		return nil
	}

	return func(ctx context.Context, w http.ResponseWriter) {
		pbReq := reflect.Indirect(reflect.New(m.RequestType().Elem())).Addr().Interface().(proto.Message)
		if err := s.decodeBody(r, pbReq); err != nil && err != io.EOF {
			httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.InvalidArgument, "http mux decodeBody failed, err = %v", err))
		} else {
			ctx = context.WithValue(ctx, HttpInvokeCtx, true)
			m.Invoke(ctx, pbReq, func(resp interface{}) (interface{}, error) { return resp, nil }, s.opts.GrpcGatewayCompatible, s.marshaler, w)
		}
	}
}

func (s *ServerMux) makeRawPostRequest(r *http.Request) (func(ctx context.Context, w http.ResponseWriter), error) {
	if r.Method != "POST" {
		return nil, nil
	}

	meta, err := s.rawPostMatcher.MatchWithMeta(r, http2.EstimateHeader)
	if err != nil || len(meta) == 0 {
		return nil, fmt.Errorf("http mux: makeRawPostRequest failed, err = %v", err)
	}

	return s.makeRawPostHandler(r, meta[0].(*_RawDesc).method), nil
}

func (s *ServerMux) makeHttpRuleHandler(r *http.Request, rd *_HttpRuleDesc, pathParams map[string]string) func(ctx context.Context, w http.ResponseWriter) {
	return func(ctx context.Context, w http.ResponseWriter) {
		pbReq := reflect.Indirect(reflect.New(rd.method.RequestType().Elem())).Addr().Interface().(proto.Message)

		if rd.bodyField != "" {
			pbField := pbReq.(interface{})
			if rd.bodyField != "*" {
				fieldVal := reflect.ValueOf(pbReq).Elem().FieldByName(generator.CamelCase(rd.bodyField))
				if !fieldVal.IsValid() {
					httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.InvalidArgument, "no %q field found in %s", rd.bodyField, rd.method.RequestType().Elem()))
					return
				}
				pbField = fieldVal.Addr().Interface()
			}
			if err := s.decodeBody(r, pbField); err != nil && err != io.EOF {
				httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.InvalidArgument, "http mux decodeBody failed, err = %v", err))
				return
			}
		}

		for k, v := range pathParams {
			if err := runtime.PopulateFieldFromPath(pbReq, k, v); err != nil {
				httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.InvalidArgument, "http mux populateFieldFromPath failed, err = %v", err))
				return
			}
		}

		if rd.bodyField != "*" {
			if err := runtime.PopulateQueryParameters(pbReq, r.URL.Query(), rd.filter); err != nil {
				httpErrorHandler(0, w, s.marshaler, status.Errorf(codes.InvalidArgument, "http mux populateQueryParameters failed, err = %v", err))
				return
			}
		}

		if rd.parameterHandler != nil {
			for k, v := range r.URL.Query() {
				if handler, ok := rd.parameterHandler[k]; ok && handler != nil {
					if newReq, ok := handler(r, v, pbReq); ok {
						pbReq = newReq
					}
				}
			}
		}

		ctx = context.WithValue(ctx, HttpInvokeCtx, true)
		rd.method.Invoke(ctx, pbReq, func(resp interface{}) (interface{}, error) {
			if rd.respField != "*" && rd.respField != "" {
				fieldVal := reflect.ValueOf(resp).Elem().FieldByName(generator.CamelCase(rd.respField))
				if !fieldVal.IsValid() {
					return nil, status.Errorf(codes.InvalidArgument, "no %q field found in %s", rd.respField, rd.method.ResponseType().Elem())
				}
				return fieldVal.Addr().Interface(), nil
			} else {
				return resp, nil
			}
		}, s.opts.GrpcGatewayCompatible, s.marshaler, w)
	}
}

func (s *ServerMux) makeHttpRuleRequest(r *http.Request) (func(ctx context.Context, w http.ResponseWriter), error) {
	if !strings.HasPrefix(r.URL.Path, "/") {
		return nil, fmt.Errorf("http mux: makeHttpRuleRequest: not a valid path")
	}

	var verb string
	components := strings.Split(r.URL.Path[1:], "/")
	l := len(components)
	if idx := strings.LastIndex(components[l-1], ":"); idx == 0 {
		return nil, fmt.Errorf("http mux: makeHttpRuleRequest: not a valid path")
	} else if idx > 0 {
		c := components[l-1]
		components[l-1], verb = c[:idx], c[idx+1:]
	}

	pathParamsMap := make(map[interface{}]map[string]string)
	CheckPathParam := func(r *http.Request, meta interface{}, count int) error {
		rd, ok := meta.(*_HttpRuleDesc)
		if !ok {
			return fmt.Errorf("http mux: checkPathParam: meta isn't a *_HttpRuleDesc object")
		}

		pathParams, err := rd.pattern.Match(components, verb)
		if err != nil {
			return fmt.Errorf("http mux: checkPathParam: pattern match with request failed, err = %v", err)
		}

		pathParamsMap[meta] = pathParams
		return nil
	}

	meta, err := s.ruleMatcher.MatchWithMeta(r, http2.EstimateMetaChain(CheckPathParam, http2.EstimateHeader))
	if err != nil || len(meta) == 0 {
		return nil, fmt.Errorf("http mux: makeHttpRuleRequest failed, err = %v", err)
	}

	rd := meta[0].(*_HttpRuleDesc)
	return s.makeHttpRuleHandler(r, rd, pathParamsMap[meta[0]]), nil
}

func httpErrorHandler(code int, w http.ResponseWriter, marshaler *runtime.JSONPb, err error) {
	if code == 0 {
		const fallback = `{"code": 13, "message": "failed to marshal error message"}`

		w.Header().Set("Content-Type", "application/json")
		stat, ok := status.FromError(err)
		if !ok {
			stat = status.New(codes.Unknown, err.Error())
		}

		w.Header().Del("Trailer")

		resp, merr := marshaler.Marshal(stat.Proto())
		if merr != nil {
			log.Errorf("Failed to marshal error message %q: %v\n", stat.Proto(), merr)
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := io.WriteString(w, fallback); err != nil {
				log.Errorf("[ERROR] Failed to write response: %v\n", err)
			}
			return
		}

		st := httpStatusFromCode(stat.Code())
		w.WriteHeader(st)
		if _, err := w.Write(resp); err != nil {
			log.Errorf("[ERROR] Failed to write response: %v\n", err)
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Del("Trailer")
		w.WriteHeader(code)
	}
}

func (s *ServerMux) decodeBody(r *http.Request, v interface{}) error {
	switch v := v.(type) {
	case *[]byte:
		if body, err := ioutil.ReadAll(r.Body); err != nil {
			if err == io.EOF {
				return err
			} else {
				return fmt.Errorf("serverMux decodeBody readAll http body failed, err = %+v", err)
			}
		} else {
			*v = body
			return nil
		}
	case *string:
		if body, err := ioutil.ReadAll(r.Body); err != nil {
			if err == io.EOF {
				return err
			} else {
				return fmt.Errorf("serverMux decodeBody readAll http body failed, err = %+v", err)
			}
		} else {
			*v = string(body)
			return nil
		}
	default:
		if newReader, err := utilities.IOReaderFactory(r.Body); err != nil {
			if err == io.EOF {
				return err
			} else {
				return fmt.Errorf("serverMux decodeBody create IOReaderFactory failed, err = %+v", err)
			}
		} else {
			if err := s.marshaler.NewDecoder(newReader()).Decode(v); err != nil {
				if err == io.EOF {
					return err
				} else {
					return fmt.Errorf("serverMux decodeBody unmarshal request body failed, err = %+v", err)
				}
			} else {
				return nil
			}
		}
	}
}

func (s *ServerMux) methodPatternString(method, path, body, resp string) string {
	var msg bytes.Buffer

	msg.WriteString(method)
	msg.WriteString(" Path: ")
	msg.WriteString(path)
	if body != "" {
		msg.WriteString(" | Body: \"")
		msg.WriteString(body)
		msg.WriteString("\"")
	}
	if resp != "" {
		msg.WriteString(" | Response-Body: \"")
		msg.WriteString(resp)
		msg.WriteString("\"")
	}
	return msg.String()
}

func (s *ServerMux) HttpRules() []*registry.HttpRule {
	return s.httpRules
}
