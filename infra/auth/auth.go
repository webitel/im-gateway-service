package auth

import (
	"context"
	"strings"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc/metadata"
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
	// ViaTypeIdentificationHeader carries the provider type of the gate (facebook | whatsapp | telegram_bot | ...).
	ViaTypeIdentificationHeader = "x-webitel-via-type"
	// ViaNameIdentificationHeader carries the gate display name. Binary (-bin) metadata:
	// gRPC base64-encodes it on the wire, so the value read here is already the raw bytes.
	ViaNameIdentificationHeader = "x-webitel-via-name-bin"
	// QueueIDIdentificationHeader carries the call_center queue id (int64) on member/preview calls.
	QueueIDIdentificationHeader = "x-webitel-queue-id"
	// QueueNameIdentificationHeader carries the call_center queue display name. Binary (-bin) metadata,
	// like ViaNameIdentificationHeader.
	QueueNameIdentificationHeader = "x-webitel-queue-name-bin"
)

// BinaryHeader returns the first value of a binary (-bin) header. The value may hold
// arbitrary bytes, so it is coerced to valid UTF-8: proto3 string fields reject invalid
// UTF-8 on marshal.
func BinaryHeader(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}

	return strings.ToValidUTF8(vals[0], "\uFFFD")
}

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
	GetViaName() string
	GetViaType() string
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
