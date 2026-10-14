package mapper_test

import (
	"testing"

	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
)

func TestMapToReadMessageRequest_UpToSeq(t *testing.T) {
	got := mapper.MapToReadMessageRequest(&pb.ReadMessageRequest{ThreadId: "t1", UpToSeq: 9})

	if got.ThreadID != "t1" || got.UpToSeq != 9 || got.MessageID != "" {
		t.Fatalf("dto = %+v, want thread t1 up to seq 9", got)
	}
}
