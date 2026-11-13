package grpc

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service"
)

// retryOfMessenger records the retry_of the handler hands to the service.
type retryOfMessenger struct {
	service.Messenger

	got *string
}

func (m *retryOfMessenger) SendContact(_ context.Context, in *impb.SendContactRequest) (*impb.SendMessageResponse, error) {
	m.got = in.RetryOf

	return &impb.SendMessageResponse{}, nil
}

func (m *retryOfMessenger) SendLocation(_ context.Context, in *impb.SendLocationRequest) (*impb.SendMessageResponse, error) {
	m.got = in.RetryOf

	return &impb.SendMessageResponse{}, nil
}

// Contact and location have no mapper: the handler passes the request through,
// so retry_of must survive it unchanged.
func TestMessageService_RetryOfPassthrough(t *testing.T) {
	t.Parallel()

	retryOf := uuid.NewString()

	tests := []struct {
		name string
		send func(h *MessageService, retryOf *string) error
	}{
		{"contact", func(h *MessageService, r *string) error {
			_, err := h.SendContact(context.Background(), &impb.SendContactRequest{RetryOf: r})

			return err
		}},
		{"location", func(h *MessageService, r *string) error {
			_, err := h.SendLocation(context.Background(), &impb.SendLocationRequest{RetryOf: r})

			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []*string{proto.String(retryOf), nil} {
				m := &retryOfMessenger{}
				h := NewMessageService(slog.New(slog.NewTextHandler(io.Discard, nil)), m)

				if err := tc.send(h, want); err != nil {
					t.Fatalf("send: %v", err)
				}

				if (want == nil) != (m.got == nil) || (want != nil && *m.got != *want) {
					t.Errorf("retry_of = %v, want %v", m.got, want)
				}
			}
		})
	}
}
