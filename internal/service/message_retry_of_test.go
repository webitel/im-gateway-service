package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/infra/auth/standard"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// fakeThreadSender records the retry_of of each send that reaches im-thread.
type fakeThreadSender struct {
	threadv1.MessageClient

	got *string
}

func (f *fakeThreadSender) SendText(_ context.Context, in *threadv1.SendTextRequest, _ ...grpc.CallOption) (*threadv1.SendTextResponse, error) {
	f.got = in.RetryOf

	return &threadv1.SendTextResponse{Id: uuid.NewString()}, nil
}

func (f *fakeThreadSender) SendDocument(_ context.Context, in *threadv1.SendDocumentRequest, _ ...grpc.CallOption) (*threadv1.SendDocumentResponse, error) {
	f.got = in.RetryOf

	return &threadv1.SendDocumentResponse{Id: uuid.NewString()}, nil
}

func (f *fakeThreadSender) SendContact(_ context.Context, in *threadv1.SendContactRequest, _ ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	f.got = in.RetryOf

	return &threadv1.SendMessageResponse{}, nil
}

func (f *fakeThreadSender) SendLocation(_ context.Context, in *threadv1.SendLocationRequest, _ ...grpc.CallOption) (*threadv1.SendMessageResponse, error) {
	f.got = in.RetryOf

	return &threadv1.SendMessageResponse{}, nil
}

func TestMessageService_RetryOfReachesThread(t *testing.T) {
	t.Parallel()

	threadID := uuid.NewString()
	toDTO := shared.Peer{Type: shared.PeerThread, ID: threadID}
	toProto := &api.Peer{Kind: &api.Peer_ThreadId{ThreadId: threadID}}

	tests := []struct {
		name string
		send func(ctx context.Context, m *MessageService, retryOf *string) error
	}{
		{"text", func(ctx context.Context, m *MessageService, r *string) error {
			_, err := m.SendText(ctx, &dto.SendTextRequest{To: toDTO, Body: "hi", RetryOf: r})

			return err
		}},
		{"document", func(ctx context.Context, m *MessageService, r *string) error {
			_, err := m.SendDocument(ctx, &dto.SendDocumentRequest{To: toDTO, RetryOf: r})

			return err
		}},
		{"contact", func(ctx context.Context, m *MessageService, r *string) error {
			_, err := m.SendContact(ctx, &api.SendContactRequest{To: toProto, RetryOf: r})

			return err
		}},
		{"location", func(ctx context.Context, m *MessageService, r *string) error {
			_, err := m.SendLocation(ctx, &api.SendLocationRequest{To: toProto, RetryOf: r})

			return err
		}},
	}

	ctx := context.WithValue(context.Background(), auth.AuthContextKey, auth.Identifier(&standard.Identity{}))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, want := range []*string{proto.String(uuid.NewString()), nil} {
				thread := &fakeThreadSender{}
				m := &MessageService{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), threader: thread}

				if err := tc.send(ctx, m, want); err != nil {
					t.Fatalf("send: %v", err)
				}

				if (want == nil) != (thread.got == nil) || (want != nil && *thread.got != *want) {
					t.Errorf("retry_of = %v, want %v", thread.got, want)
				}
			}
		})
	}
}
