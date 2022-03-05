package test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/zhangel/go-framework.git"
	"github.com/zhangel/go-framework.git/dialer"
	"github.com/zhangel/go-framework.git/log"
	"github.com/zhangel/go-framework.git/server"
	"github.com/zhangel/go-framework.git/testpb"
)

type TestSrv struct {
	server.ServiceProvider
}

func (s *TestSrv) Unary(ctx context.Context, req *testpb.Request) (*testpb.Response, error) {
	logger := log.WithContext(ctx)
	logger.Tracef("Unary, request = %+v", req)
	return &testpb.Response{Data: "Unary response", Code: 0}, nil
}

func (s *TestSrv) ServerSideStreaming(req *testpb.Request, streamer testpb.TestApi_ServerSideStreamingServer) error {
	logger := log.WithContext(streamer.Context())
	logger.Tracef("ServerSideStreaming, request = %+v", req)

	metadata.NewOutgoingContext(streamer.Context(), metadata.New(map[string]string{"TEST": "TEST"}))

	md := metadata.New(map[string]string{
		"Header1": "Value1",
	})
	streamer.SetHeader(md)
	streamer.SetTrailer(md)
	for i := 0; i < 10; i++ {
		logger.Tracef("ServerSideStreaming, Send response for %d", i)
		err := streamer.Send(&testpb.Response{Data: fmt.Sprintf("ServerSideStreaming response for %d", i), Code: int32(i)})
		if err != nil {
			logger.Errorf("ServerSideStreaming server::Send failed, err = %v", err)
			break
		}
	}
	return nil
}

func (s *TestSrv) ClientSideStreaming(streamer testpb.TestApi_ClientSideStreamingServer) error {
	logger := log.WithContext(streamer.Context())

	i := 0
	for {
		req, err := streamer.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			logger.Error(err)
		} else {
			i++
			log.Trace("ClientSideStreaming:", req)
		}
	}

	err := streamer.SendAndClose(&testpb.Response{Data: fmt.Sprintf("ClientSideStreaming response, got %d request", i), Code: 0})
	if err != nil {
		logger.Errorf("ClientSideStream server::SendAndClose failed, err = %v", err)
	}

	return nil
}

func (s *TestSrv) BidiSideStreaming(streamer testpb.TestApi_BidiSideStreamingServer) error {
	logger := log.WithContext(streamer.Context())

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			req, err := streamer.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				logger.Error(err)
			} else {
				log.Trace("BidiSideStreaming", req)
			}
		}
	}()

	for i := 0; i < 5; i++ {
		logger.Tracef("BidiStreaming, Send response for %d", i)
		err := streamer.Send(&testpb.Response{Data: fmt.Sprintf("BidiStreaming response for %d", i), Code: int32(i)})
		if err != nil {
			if err != io.EOF {
				logger.Error(err)
			}
			break
		}
		logger.Tracef("BidiStreaming, Send response for %d successful", i)
	}

	wg.Wait()
	return nil
}

func TestMain(m *testing.M) {
	defer framework.Init()()
	if err := server.RegisterService(testpb.RegisterTestApiServer, &TestSrv{}); err != nil {
		log.Fatal(err)
	}
	go server.Run()
	m.Run()
}

func TestServerModule(t *testing.T) {
	cli := testpb.NewTestApiClient(dialer.ClientConn)
	if resp, err := cli.Unary(context.Background(), &testpb.Request{Data: "Hello"}); err != nil {
		t.Fatal("Unary failed, err = ", err)
	} else {
		t.Log("Unary resp =", resp)
	}

	if stream, err := cli.ServerSideStreaming(context.Background(), &testpb.Request{Data: "ServerStreaming"}); err != nil {
		t.Fatal("Server-side streaming failed, err = ", err)
	} else {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				t.Log("Server-side streaming done")
				break
			} else if err != nil {
				t.Fatal("Server-side streaming recv failed, err =", err)
			} else {
				t.Log("Server-side streaming recv resp =", resp)
			}
		}
	}

	if stream, err := cli.ClientSideStreaming(context.Background()); err != nil {
		t.Fatal("Client-side streaming failed, err = ", err)
	} else {
		for i := 0; i < 10; i++ {
			if err := stream.SendMsg(&testpb.Request{Data: "Hello"}); err != nil {
				t.Fatal("Client-side streaming sendMsg failed, err =", err)
			}
		}

		if resp, err := stream.CloseAndRecv(); err != nil {
			t.Fatal("Client-side streaming CloseSend failed, err =", err)
		} else {
			t.Log("Client-side streaming recv resp = ", resp)
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	if stream, err := cli.BidiSideStreaming(context.Background()); err != nil {
		t.Fatal("BidiStreaming failed, err =", err)
	} else {
		go func() {
			defer wg.Done()
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				} else if err != nil {
					t.Fatal("BidiStreaming recv failed, err =", err)
				} else {
					t.Log("Bidi streaming recv resp =", resp)
				}
			}
		}()

		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			if err := stream.SendMsg(&testpb.Request{Data: "Hello"}); err != nil {
				t.Fatal("BidiStreaming sendMsg failed, err =", err)
			}
		}
		if err := stream.CloseSend(); err != nil {
			t.Fatal("BidiStreaming CloseSend failed, err =", err)
		}
	}
	wg.Wait()
}
