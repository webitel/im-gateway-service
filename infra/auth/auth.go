package auth

import (
	"context"

	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var (
	IdentityNotFoundErr = errors.Forbidden("identity not found in the context")
	ForbiddenIssuerErr  = errors.Forbidden("forbidden issuer")
)

type contextKey string

const (
	AuthContextKey     contextKey = "auth_identity"
	ViaContextKey      contextKey = "via"
	AuthTypeContextKey contextKey = "auth_type"

	// Headers for internal identification
	SchemaIdentificationHeader   = "x-webitel-schema"
	ProviderIdentificationHeader = "x-webitel-provider"
	XWebitelTypeHeader           = "x-webitel-type"
	ViaIdentificationHeader      = "x-webitel-via"
)

type XWebitelType string

const (
	XWebitelTypeSchema   XWebitelType = "schema"
	XWebitelTypeEngine   XWebitelType = "engine"
	XWebitelTypeProvider XWebitelType = "provider"
)

type Authorizer interface {
	SetIdentity(ctx context.Context) (context.Context, error)
}

type Identifier interface {
	GetContactID() string
	GetDomainID() int64
	GetIssuer() string
	GetName() string
	GetChatName() string
	GetApplicationID() string
	GetVia() string
	GetViaPtr() *string
}

func GetIdentityFromContext(ctx context.Context) (Identifier, bool) {
	id, ok := ctx.Value(AuthContextKey).(Identifier)
	return id, ok
}

func GetViaFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ViaContextKey).(string)
	return v
}

// WithAuthType stores the transport auth type resolved during authentication.
// It is only set for verified mTLS service calls (see resolveServiceIdentity).
func WithAuthType(ctx context.Context, t XWebitelType) context.Context {
	return context.WithValue(ctx, AuthTypeContextKey, t)
}

// AuthTypeFromContext returns the transport auth type resolved during
// authentication, if any. It is absent for regular end-user requests.
func AuthTypeFromContext(ctx context.Context) (XWebitelType, bool) {
	t, ok := ctx.Value(AuthTypeContextKey).(XWebitelType)
	return t, ok
}

// IsSystemCall reports whether the request came from a trusted service
// orchestrator (schema — flow_manager/call_center — or engine) over mTLS. Only
// such calls may use the im-thread system path that adds/removes members
// without a thread-member initiator; regular user and provider calls always
// carry an initiator so membership/permission checks are enforced.
func IsSystemCall(ctx context.Context) bool {
	t, ok := AuthTypeFromContext(ctx)
	return ok && (t == XWebitelTypeSchema || t == XWebitelTypeEngine)
}
