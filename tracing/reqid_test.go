package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/metadata"
)

type ReqidTestSuite struct {
	suite.Suite
}

func (s *ReqidTestSuite) TestNullTraceInfo() {
	ti, have := TracingInfoFromMetadata(context.Background())
	s.True(!have, "have must be false")
	s.Empty(ti.rid, "", "rid must be empty")
	s.Empty(ti.cid, "", "cid must be empty")
	s.Empty(ti.pid, "", "pid must be empty")
	s.Empty(ti.rpc, "", "rpc must be empty")
	s.Empty(ti.prpc, "", "prpc must be empty")
}

func (s *ReqidTestSuite) TestExceptionTraceInfo() {
	rid := "req-123"
	md := metadata.New(nil)
	md.Set(tagRid, rid)
	//md.Set(_TAG_CID, tracingInfo.cid)
	//md.Set(_TAG_PID, tracingInfo.pid)
	//md.Set(_TAG_RPC, tracingInfo.rpc)
	//md.Set(_TAG_PRPC, tracingInfo.prpc)
	//md.Set(_TAG_REQ_ST, strconv.FormatInt(time.Now().UnixNano(), 10))

	ctx := metadata.NewIncomingContext(context.Background(), md)
	ti, have := TracingInfoFromMetadata(ctx)

	s.True(have, "have must be false")
	s.Equal(rid, ti.rid, "", "rid must be equal")
	s.Empty(ti.cid, "", "cid must be empty")
	s.Empty(ti.pid, "", "pid must be empty")
	s.Empty(ti.rpc, "", "rpc must be empty")
	s.Empty(ti.prpc, "", "prpc must be empty")
}

func (s *ReqidTestSuite) TestValidTraceInfo() {
	tracingInfo := &TracingInfo{
		rid:  "req-123",
		cid:  "cid-123",
		pid:  "pid-123",
		rpc:  "rpc-123",
		prpc: "rpc-0",
	}

	ctx := TracingInfoToMetadata(context.Background(), tracingInfo, time.Now().Unix())

	ti, have := TracingInfoFromMetadata(ctx)
	s.True(have, "have must be true")
	s.Equal(tracingInfo.rid, ti.rid, "", "rid must be equal")
	s.Equal(tracingInfo.cid, ti.cid, "", "cid must be equal")
	s.Equal(tracingInfo.pid, ti.pid, "", "pid must be equal")
	s.Equal(tracingInfo.rpc, ti.rpc, "", "rpc must be equal")
	s.Equal(tracingInfo.prpc, ti.prpc, "", "prpc must be equal")
}

func TestReqidTestSuite(t *testing.T) {
	s := &ReqidTestSuite{}
	suite.Run(t, s)
}
