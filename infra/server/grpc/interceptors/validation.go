package interceptors

import (
	"context"

	"buf.build/go/protovalidate"
	validatemiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func ValidationInterceptor(validator protovalidate.Validator) grpc.UnaryServerInterceptor {
	baseInterceptor := validatemiddleware.UnaryServerInterceptor(validator)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := baseInterceptor(ctx, req, info, handler)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				return nil, errors.New(st.Message(), errors.WithCause(err), errors.WithCode(st.Code()), errors.WithID("interceptors.validation.validation_interceptor"))
			}

			return nil, err
		}

		return resp, nil
	}
}
