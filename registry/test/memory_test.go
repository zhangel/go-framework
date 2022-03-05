package test

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/zhangel/go-framework.git"
	"github.com/zhangel/go-framework.git/dialer"
	"github.com/zhangel/go-framework.git/registry"
	"github.com/zhangel/go-framework.git/server"
	"github.com/zhangel/go-framework.git/testpb"
)

const (
	serviceName = "test.pb.TestApi"
	serverAddr  = "127.0.0.1:9400"
)

type TestSrv struct {
	server.ServiceProvider
}

func (s *TestSrv) Unary(ctx context.Context, req *testpb.Request) (*testpb.Response, error) {
	return &testpb.Response{Data: "Unary response", Code: 0}, nil
}

func (s *TestSrv) ServerSideStreaming(req *testpb.Request, streamer testpb.TestApi_ServerSideStreamingServer) error {
	return nil
}

func (s *TestSrv) ClientSideStreaming(streamer testpb.TestApi_ClientSideStreamingServer) error {
	return nil
}

func (s *TestSrv) BidiSideStreaming(streamer testpb.TestApi_BidiSideStreamingServer) error {
	return nil
}

func TestMain(m *testing.M) {
	defer framework.Init()()
	if err := server.RegisterService(testpb.RegisterTestApiServer, &TestSrv{}); err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := server.Run(server.WithAddr(serverAddr)); err != nil {
			log.Fatal(err)
		}
	}()

	m.Run()
}

func TestMemoryRegistry(t *testing.T) {
	memoryRegistry, err := registry.NewMemoryServiceDiscovery()
	if err != nil {
		t.Fatalf("NewMemoryRegistry failed, err = %v", err)
	}

	testClient := testpb.NewTestApiClient(dialer.ClientConn)
	if err := dialer.RegisterDialOption("TestDialer", dialer.DialWithRegistry(memoryRegistry), dialer.DialWithCustomDialer(func(ctx context.Context, addr string) (conn net.Conn, err error) {
		return net.Dial("tcp", addr)
	})); err != nil {
		t.Fatalf("dialer.RegisterDialOption failed, err = %v", err)
	}

	if _, err := testClient.Unary(context.Background(), &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Logf("unary call failed because unregister, err = %v", err)
	} else {
		t.Fatal("unary call should failed because unregister, but done")
	}

	if err := memoryRegistry.RegisterService(serviceName, serverAddr, nil); err != nil {
		t.Fatalf("RegisterService failed, err = %v", err)
	}

	memoryRegistry.WatchAllServiceChanges(registry.NewWatcherWrapper(func(serviceName string, instances []registry.Instance) {
		t.Logf("OnUpdateCallback, serviceName = %s, instances = %v", serviceName, instances)
	}, func(serviceName string, instances []registry.Instance) {
		t.Logf("OnDeleteCallback, serviceName = %s, instances = %v", serviceName, instances)
	}))

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	if resp, err := testClient.Unary(ctx, &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Fatal("Unary failed, err = ", err)
	} else {
		t.Log("Unary resp =", resp)
	}

	if err := memoryRegistry.UnregisterService(serviceName, serverAddr); err != nil {
		t.Fatalf("UnregisterService failed, err = %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if _, err := testClient.Unary(context.Background(), &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Logf("unary call failed because unregister, err = %v", err)
	} else {
		t.Fatal("unary call should failed because unregister, but done")
	}

	if err := memoryRegistry.RegisterServiceWithEntries([]string{serviceName}, []registry.Instance{{Addr: serverAddr}}); err != nil {
		t.Fatalf("RegisterService failed, err = %v", err)
	}

	if resp, err := testClient.Unary(context.Background(), &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Fatal("Unary failed, err = ", err)
	} else {
		t.Log("Unary resp =", resp)
	}

	if err := memoryRegistry.RegisterServiceWithEntries([]string{serviceName}, []registry.Instance{{Addr: "127.0.0.1:1234"}}); err != nil {
		t.Fatalf("RegisterService failed, err = %v", err)
	}

	if _, err := testClient.Unary(context.Background(), &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Logf("unary call failed because invalid address, err = %v", err)
	} else {
		t.Fatal("unary call should failed because unregister, but done")
	}

	if err := memoryRegistry.RegisterServiceWithEntries([]string{serviceName}, []registry.Instance{{Addr: "127.0.0.1:1234"}, {Addr: serverAddr}}); err != nil {
		t.Fatalf("RegisterService failed, err = %v", err)
	}

	if resp, err := testClient.Unary(context.Background(), &testpb.Request{Data: "Hello"}, dialer.WithDialOptTag("TestDialer"), dialer.WithUseInProcDial(false)); err != nil {
		t.Fatal("Unary failed, err = ", err)
	} else {
		t.Log("Unary resp =", resp)
	}

}
