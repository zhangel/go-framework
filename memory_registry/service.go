package memory_registry

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"google.golang.org/grpc"
)

type ServiceInfo struct {
	ServiceName string
	HandlerType interface{}
	Methods     map[string]*grpc.MethodDesc
	Streams     map[string]*grpc.StreamDesc
	MetaData    interface{}
}

func RegisterService(server *grpc.Server, register, srvHandler interface{}) error {
	rt := reflect.TypeOf(register)
	if register == nil || srvHandler == nil {
		return fmt.Errorf("[ERROR] Invalid service register parameters.\n")
	}
	if reflect.TypeOf(register).Kind() != reflect.Func || rt.NumIn() != 2 || rt.In(0) != reflect.TypeOf(server) {
		return fmt.Errorf("[ERROR] Invalid service register function[%s].\n", rt.String())
	}
	if !reflect.TypeOf(srvHandler).Implements(rt.In(1)) {
		return fmt.Errorf("[ERROR] Register service failed, %s doesn't implements %s interface.\n", reflect.TypeOf(srvHandler).String(), rt.In(1).String())
	}

	reflect.ValueOf(register).Call([]reflect.Value{reflect.ValueOf(server), reflect.ValueOf(srvHandler)})
	return nil
}

func GetServiceInfo(register, srvHandler interface{}) (serviceInfo *ServiceInfo, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("FetchServiceInfo failed, e = %+v", e)
		}
	}()

	srv := grpc.NewServer()
	if err := RegisterService(srv, register, srvHandler); err != nil {
		return nil, err
	}

	serviceName := ""
	for name := range srv.GetServiceInfo() {
		serviceName = strings.TrimSpace(name)
		break
	}

	if serviceName == "" {
		return nil, fmt.Errorf("FetchServiceInfo failed, serviceName is nil")
	}

	serviceInfo = &ServiceInfo{
		ServiceName: serviceName,
		HandlerType: nil,
		Methods:     make(map[string]*grpc.MethodDesc),
		Streams:     make(map[string]*grpc.StreamDesc),
		MetaData:    nil,
	}

	mField := reflect.ValueOf(srv).Elem().FieldByName("m")
	serviceField := mField.MapIndex(reflect.ValueOf(serviceInfo.ServiceName)).Elem()

	serverField := serviceField.FieldByName("server")
	serverField = reflect.NewAt(serverField.Type(), unsafe.Pointer(serverField.UnsafeAddr())).Elem()
	serviceInfo.HandlerType = serverField.Interface()

	mdField := serviceField.FieldByName("md")
	mdField = reflect.NewAt(mdField.Type(), unsafe.Pointer(mdField.UnsafeAddr())).Elem()
	for _, key := range mdField.MapKeys() {
		reflect.ValueOf(serviceInfo.Methods).SetMapIndex(key, mdField.MapIndex(key))
	}

	sdField := serviceField.FieldByName("sd")
	sdField = reflect.NewAt(sdField.Type(), unsafe.Pointer(sdField.UnsafeAddr())).Elem()
	for _, key := range sdField.MapKeys() {
		reflect.ValueOf(serviceInfo.Streams).SetMapIndex(key, sdField.MapIndex(key))
	}

	mdataField := serviceField.FieldByName("mdata")
	mdataField = reflect.NewAt(mdataField.Type(), unsafe.Pointer(mdataField.UnsafeAddr())).Elem()
	serviceInfo.MetaData = mdataField.Interface()

	return serviceInfo, nil
}
