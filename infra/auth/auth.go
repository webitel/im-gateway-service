package auth

import (
	"context"

	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var IdentityNotFoundErr = errors.New("identity not found in the context")

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
	GetName() string
	GetVia() string
}

func GetIdentityFromContext(ctx context.Context) (Identifier, bool) {
	id, ok := ctx.Value(AuthContextKey).(Identifier)
	return id, ok
}

func GetViaFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ViaContextKey).(string)
	return v
}
