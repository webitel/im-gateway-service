package imauth

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type tokenError struct {
	id   string
	code codes.Code
}

// The go-kit error interceptor ignores errors.WithID and passes err.Error() to
// the frontend as "detail" unchanged, so the id is sent as the message.
var specialProcessesTokenErrors = map[string]tokenError{
	"CAPTCHA_REQUIRED": {id: "api.auth.captcha_required", code: codes.Unauthenticated},
	"CAPTCHA_FAILED":   {id: "api.auth.captcha_failed", code: codes.Unauthenticated},
}

// Reveal internal state: logged only, the frontend gets a generic error.
var loggedTokenErrors = map[string]struct{}{
	"CAPTCHA_UNAVAILABLE":   {},
	"CAPTCHA_MISCONFIGURED": {},
}

const errTokenServerMessage = "internal server error"

// processTokenError maps im-account-service Token errors to the ones returned to the frontend.
func processTokenError(ctx context.Context, logger *slog.Logger, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	if te, ok := specialProcessesTokenErrors[st.Message()]; ok {
		return errors.New(te.id, errors.WithCode(te.code), errors.WithID(te.id), errors.WithCause(err))
	}

	if _, ok := loggedTokenErrors[st.Message()]; ok {
		logger.ErrorContext(ctx, "token failed on the server side",
			slog.String("status", st.Message()),
			slog.String("grpc_code", st.Code().String()),
		)

		return errors.Internal(errTokenServerMessage, errors.WithCause(err))
	}

	return err
}
