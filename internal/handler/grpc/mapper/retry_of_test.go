package mapper_test

import (
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
)

func TestMapSendRequests_RetryOf(t *testing.T) {
	t.Parallel()

	retryOf := uuid.NewString()

	tests := []struct {
		name string
		got  func(retryOf *string) *string
	}{
		{"text", func(r *string) *string {
			return mapper.MapToSendTextRequest(&pb.SendTextRequest{Body: "hi", RetryOf: r}).RetryOf
		}},
		{"document", func(r *string) *string {
			return mapper.MapToSendDocumentRequest(&pb.SendDocumentRequest{RetryOf: r}).RetryOf
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.got(proto.String(retryOf)); got == nil || *got != retryOf {
				t.Errorf("retry_of = %v, want %q", got, retryOf)
			}

			if got := tc.got(nil); got != nil {
				t.Errorf("retry_of = %q, want nil when unset", *got)
			}
		})
	}
}
