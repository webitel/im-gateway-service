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
	AuthContextKey contextKey = "auth_identity"
	ViaContextKey  contextKey = "via"

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
