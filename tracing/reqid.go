package tracing

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/utils"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/zhangel/go-framework/log"
	"github.com/zhangel/go-framework/log/fields"
)

const (
	tagRid   = "rid"
	tagCid   = "cid"
	tagPid   = "pid"
	tagRpc   = "rpc"
	tagPrpc  = "prpc"
	tagReqSt = "reqst"

	tagHttpRid = "X-Request-Id"
	tagHttpCid = "X-Call-Id"
	tagHttpRpc = "X-Rpc"

	lanxinCallstack2 = "callstack2"
)

type ReqIdCallOpt struct {
	grpc.EmptyCallOption
	reqId string
}

func WithReqId(reqId string) grpc.CallOption {
	return &ReqIdCallOpt{reqId: reqId}
}

type TracingInfo struct {
	// request id，对应open-tracing中的traceid(概念对应，但值不对应，下同)，对于一个请求链始终保持不变
	rid string
	// call id，对应open-tracing中的spanid，每次grpc请求生成一个新的id
	cid string
	// parent id，对应open-tracing中的parentid，值为当前服务调用者的cid
	pid string
	// rpc method，当前服务的rpc method name
	rpc string
	// parent rpc method，当前服务调用者的rpc method name
	prpc string
}

type LanxinCallStack struct {
	ReqId    string
	PRPC     string
	PID      string
	ID       string
	RPC      string
	Cid      int
	Rid      int
	CallTime int64
	Begin    int64
	End      int64
}

type tracingInfoReader func(md metadata.MD) (*TracingInfo, bool)
type tracingInfoWriter func(ctx context.Context, tracingInfo *TracingInfo, md metadata.MD, ts int64)
type tracingInfoAdapter struct {
	reader tracingInfoReader
	writer tracingInfoWriter
}

var (
	adapters = []tracingInfoAdapter{
		{tracingInfoFromMetadata, tracingInfoToMetadata},
		{tracingInfoFromLanxinMetadata, nil},
		{tracingInfoFromLanxinCallstack2, tracingInfoToLanxinCallstack2},
	}
)

const (
	methodCtx      = 0
	tracingInfoCtx = 1
	timestampCtx   = 2
)

var tracingEnabled = true

func ContextWithReqId(ctx context.Context, reqId, pid, cid, rpc, prpc string) context.Context {
	tracingInfo := &TracingInfo{
		rid:  reqId,
		pid:  pid,
		cid:  cid,
		rpc:  rpc,
		prpc: prpc,
	}

	return context.WithValue(ctx, tracingInfoCtx, tracingInfo)
}

func RequestIdTracingUnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
	enterTime := time.Now().UnixNano()

	tracingInfo, _ := ctx.Value(tracingInfoCtx).(*TracingInfo)
	if tracingInfo == nil {
		for _, o := range opts {
			if reqIdCallOpt, ok := o.(*ReqIdCallOpt); ok {
				tracingInfo = &TracingInfo{
					rid:  reqIdCallOpt.reqId,
					pid:  "",
					cid:  reqIdCallOpt.reqId,
					rpc:  "",
					prpc: "",
				}
			}
		}
	}

	if tracingInfo == nil {
		tracingInfo = &TracingInfo{}
	}

	childTracingInfo := ChildTracingInfo(method, tracingInfo)
	ctx = TracingInfoToMetadata(ctx, childTracingInfo, enterTime)

	err = invoker(ctx, method, req, reply, cc, opts...)

	if tracingEnabled {
		log.WithFields(append(genRpcCallFields(false, enterTime, err), tracingInfoFields(childTracingInfo, enterTime)...)).Info("rpc-call")
	}

	return err
}

func RequestIdTracingStreamingClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	tracingInfo, _ := ctx.Value(tracingInfoCtx).(*TracingInfo)
	if tracingInfo == nil {
		for _, o := range opts {
			if reqIdCallOpt, ok := o.(*ReqIdCallOpt); ok {
				tracingInfo = &TracingInfo{
					rid:  reqIdCallOpt.reqId,
					pid:  "",
					cid:  reqIdCallOpt.reqId,
					rpc:  "",
					prpc: "",
				}
			}
		}
	}

	if tracingInfo == nil {
		tracingInfo = &TracingInfo{}
	}

	ctx = TracingInfoToMetadata(ctx, ChildTracingInfo(method, tracingInfo), time.Now().UnixNano())

	clientStream, err := streamer(ctx, desc, cc, method, opts...)
	return clientStream, err
}

func RequestIdTracingUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	enterTime := time.Now().UnixNano()

	ctx = context.WithValue(ctx, timestampCtx, enterTime)
	ctx = context.WithValue(ctx, methodCtx, info.FullMethod)
	tracingInfo, _ := ctx.Value(tracingInfoCtx).(*TracingInfo)
	if tracingInfo == nil {
		tracingInfo, _ = TracingInfoFromMetadata(ctx)
	}
	ctx = context.WithValue(ctx, tracingInfoCtx, tracingInfo)

	resp, err := handler(ctx, req)

	if tracingEnabled {
		log.WithContext(ctx).WithFields(genRpcCallFields(true, enterTime, err)).Info("rpc-call")
	}
	return resp, err
}

func RequestIdTracingStreamingServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()

	ctx = context.WithValue(ctx, timestampCtx, time.Now().UnixNano())
	ctx = context.WithValue(ctx, methodCtx, info.FullMethod)

	tracingInfo, _ := ctx.Value(tracingInfoCtx).(*TracingInfo)
	if tracingInfo == nil {
		tracingInfo, _ = TracingInfoFromMetadata(ctx)
	}
	ctx = context.WithValue(ctx, tracingInfoCtx, tracingInfo)

	ss = &tracingServerStream{
		ctx:          ctx,
		ServerStream: ss,
	}
	err := handler(srv, ss)
	return err
}

func HttpServerReqIdHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reqIdHeader, _ := r.Header[tagHttpRid]; len(reqIdHeader) == 0 || reqIdHeader[0] == "" {
			handler.ServeHTTP(w, r)
			return
		}

		enterTime := time.Now().UnixNano()
		ctx := context.WithValue(r.Context(), timestampCtx, enterTime)

		reqId := r.Header[tagHttpRid][0]

		callId := reqId
		if callIdHeader, _ := r.Header[tagHttpCid]; len(callIdHeader) > 0 && callIdHeader[0] != "" {
			callId = callIdHeader[0]
		}

		rpc := "/" + r.Method + r.URL.Path
		if rpcHeader, _ := r.Header[tagHttpRpc]; len(rpcHeader) > 0 && rpcHeader[0] != "" {
			rpc = rpcHeader[0]
		}

		tracingInfo := &TracingInfo{}
		tracingInfo.rid = reqId
		tracingInfo.cid = callId
		tracingInfo.pid = ""
		tracingInfo.rpc = rpc
		tracingInfo.prpc = ""
		ctx = context.WithValue(ctx, tracingInfoCtx, tracingInfo)

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

type tracingServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracingServerStream) Context() context.Context {
	return s.ctx
}

func TracingInfoFromMetadata(ctx context.Context) (*TracingInfo, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md, ok = metadata.FromOutgoingContext(ctx)
		if !ok {
			return &TracingInfo{}, false
		}
	}

	for _, adapter := range adapters {
		if adapter.reader == nil {
			continue
		}

		if tracingInfo, ok := adapter.reader(md); ok {
			return tracingInfo, true
		}
	}

	return &TracingInfo{}, false
}

func TracingInfoToMetadata(ctx context.Context, tracingInfo *TracingInfo, ts int64) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = make(metadata.MD, 10)
	} else {
		md = utils.CopyMetaData(md)
	}

	for _, adapter := range adapters {
		if adapter.writer == nil {
			continue
		}

		adapter.writer(ctx, tracingInfo, md, ts)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

func getMdItem(md metadata.MD, item string) string {
	val := md.Get(item)
	if len(val) == 0 {
		return ""
	} else {
		return val[len(val)-1]
	}
}

func tracingInfoFromMetadata(md metadata.MD) (*TracingInfo, bool) {
	if getMdItem(md, tagRid) == "" {
		return nil, false
	}

	tracingInfo := &TracingInfo{}
	tracingInfo.rid = getMdItem(md, tagRid)
	tracingInfo.cid = getMdItem(md, tagCid)
	tracingInfo.pid = getMdItem(md, tagPid)
	tracingInfo.rpc = getMdItem(md, tagRpc)
	tracingInfo.prpc = getMdItem(md, tagPrpc)

	return tracingInfo, true
}

func tracingInfoToMetadata(ctx context.Context, tracingInfo *TracingInfo, md metadata.MD, _ int64) {
	md.Set(tagRid, tracingInfo.rid)
	md.Set(tagCid, tracingInfo.cid)
	md.Set(tagPid, tracingInfo.pid)
	md.Set(tagRpc, tracingInfo.rpc)
	md.Set(tagPrpc, tracingInfo.prpc)
	md.Set(tagReqSt, strconv.FormatInt(time.Now().UnixNano(), 10))
}

func tracingInfoFromLanxinMetadata(md metadata.MD) (*TracingInfo, bool) {
	if getMdItem(md, "reqid") == "" {
		return nil, false
	}

	tracingInfo := &TracingInfo{}
	tracingInfo.rid = getMdItem(md, "reqid")
	tracingInfo.cid = getMdItem(md, tagCid)
	tracingInfo.pid = getMdItem(md, tagPid)
	tracingInfo.rpc = getMdItem(md, tagRpc)
	tracingInfo.prpc = getMdItem(md, tagPrpc)

	return tracingInfo, true
}

func tracingInfoFromLanxinCallstack2(md metadata.MD) (*TracingInfo, bool) {
	cs2 := getMdItem(md, lanxinCallstack2)
	if cs2 == "" {
		return nil, false
	}

	callstack := &LanxinCallStack{}
	err := json.Unmarshal([]byte(cs2), callstack)
	if err != nil {
		return nil, false
	}

	tracingInfo := &TracingInfo{}
	tracingInfo.rid = callstack.ReqId
	tracingInfo.cid = callstack.ID
	tracingInfo.rpc = callstack.RPC
	tracingInfo.prpc = callstack.PRPC
	tracingInfo.pid = callstack.PID

	return tracingInfo, true
}

func tracingInfoToLanxinCallstack2(ctx context.Context, tracingInfo *TracingInfo, md metadata.MD, ts int64) {
	// Callstack2 only works while incoming ctx has callstack2 meta
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || getMdItem(md, lanxinCallstack2) == "" {
		return
	}

	callstack := &LanxinCallStack{
		ReqId:    tracingInfo.rid,
		PRPC:     tracingInfo.prpc,
		PID:      tracingInfo.pid,
		ID:       tracingInfo.cid,
		RPC:      tracingInfo.rpc,
		Cid:      0,
		Rid:      0,
		CallTime: ts,
		Begin:    ts,
		End:      ts,
	}

	cs2, err := json.Marshal(callstack)
	if err != nil {
		return
	}

	md.Set(lanxinCallstack2, string(cs2))
}

func ChildTracingInfo(method string, tracingInfo *TracingInfo) *TracingInfo {
	childTracingInfo := &TracingInfo{}

	uuid, err := uuid.NewUUID()
	if err == nil {
		id := md5.Sum([]byte(uuid.String()))
		childTracingInfo.cid = hex.EncodeToString(id[:8])
	}

	childTracingInfo.rpc = method

	if len(tracingInfo.rid) == 0 {
		childTracingInfo.rid = childTracingInfo.cid
	} else {
		childTracingInfo.rid = tracingInfo.rid
	}

	if len(tracingInfo.cid) != 0 {
		childTracingInfo.pid = tracingInfo.cid
	}

	if len(tracingInfo.rpc) != 0 {
		childTracingInfo.prpc = tracingInfo.rpc
	}

	return childTracingInfo
}

func genRpcCallFields(server bool, enter int64, err error) fields.Fields {
	f := make(fields.Fields, 0, 5)
	if server {
		f = append(f, fields.Field{K: "term", V: "server"})
	} else {
		f = append(f, fields.Field{K: "term", V: "client"})
	}

	if err == nil {
		f = append(f, fields.Field{K: "code", V: "OK"})
	} else if st, ok := status.FromError(err); ok {
		f = append(f, fields.Field{K: "code", V: st.Code().String()})
	} else {
		f = append(f, fields.Field{K: "code", V: err})
	}

	leaveNano := time.Now().UnixNano()
	f = append(f, fields.Field{K: "enter", V: enter})
	f = append(f, fields.Field{K: "leave", V: leaveNano})
	f = append(f, fields.Field{K: "real", V: getTimeMsString(enter, leaveNano)})

	return f
}

func tracingInfoFields(tracingInfo *TracingInfo, timestamp int64) fields.Fields {
	if tracingInfo.rid == "" {
		return nil
	}

	f := make(fields.Fields, 0, 6)

	now := time.Now().UnixNano()
	if len(tracingInfo.rid) == 0 {
		f = append(f, fields.Field{K: "reqid", V: "-"})
	} else {
		f = append(f, fields.Field{K: "reqid", V: tracingInfo.rid})
	}
	if len(tracingInfo.cid) == 0 {
		f = append(f, fields.Field{K: "cid", V: "-"})
	} else {
		f = append(f, fields.Field{K: "cid", V: tracingInfo.cid})
	}
	if len(tracingInfo.pid) == 0 {
		f = append(f, fields.Field{K: "pid", V: "-"})
	} else {
		f = append(f, fields.Field{K: "pid", V: tracingInfo.pid})
	}
	if len(tracingInfo.rpc) == 0 {
		f = append(f, fields.Field{K: "rpc", V: "-"})
	} else {
		f = append(f, fields.Field{K: "rpc", V: tracingInfo.rpc})
	}
	if len(tracingInfo.prpc) == 0 {
		f = append(f, fields.Field{K: "prpc", V: "-"})
	} else {
		f = append(f, fields.Field{K: "prpc", V: tracingInfo.prpc})
	}

	f = append(f, fields.Field{K: "tc", V: getTimeMsString(timestamp, now)})
	return f
}

func genContextFields(ctx context.Context) fields.Fields {
	tracingInfo, _ := ctx.Value(tracingInfoCtx).(*TracingInfo)
	if tracingInfo == nil {
		return fields.Fields{}
	}

	timestamp, _ := ctx.Value(timestampCtx).(int64)
	return tracingInfoFields(tracingInfo, timestamp)
}

// 参数: begin 开始时间, end 截止时间, 单位是 ns
// 返回值: 字符串, 单位是ms, 精确到小数点后三位, 如 1.001
func getTimeMsString(beginNs, endNs int64) string {
	usedNanoSec := endNs - beginNs
	if usedNanoSec < 0 {
		usedNanoSec = 0
	}

	usedTime := float64(usedNanoSec) / 1000000
	return strconv.FormatFloat(usedTime, 'f', 3, 64)
}

func init() {
	log.RegisterContextFieldsProvider(genContextFields)
	lifecycle.LifeCycle().HookInitialize(func() {
		tracingEnabled = config.Bool(log.FlagTracingLog)
	}, lifecycle.WithName("Enable tracing log"))
}
