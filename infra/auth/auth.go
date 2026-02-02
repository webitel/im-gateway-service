package auth

import (
	"context"

	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var IdentityNotFoundErr = errors.New("identity not found in the context")

type contextKey string

const (
	// AuthContextKey is used to store/retrieve Identity from context
	AuthContextKey contextKey = "auth_identity"

	SchemaIdentificationHeader = "x-webitel-schema"
	XWebitelTypeHeader         = "x-webitel-type"
)

type XWebitelType string

const (
	XWebitelTypeSchema XWebitelType = "schema"
	XWebitelTypeEngine XWebitelType = "engine"
)

type Authorizer interface {
	SetIdentity(ctx context.Context) (context.Context, error)
}

// Identity stores the resolved domain ownership and contact ID
type Identity interface {
	GetContactID() string
	GetDomainID() int64
}

// GetIdentity is a helper to extract the identity from context safely.
func GetIdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(AuthContextKey).(Identity)

	return id, ok
}
