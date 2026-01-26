package service

import (
	"context"
	"fmt"

	authv1 "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	imauth "github.com/webitel/im-gateway-service/infra/client/im-auth"
	"google.golang.org/grpc/metadata"
)

// Auther defines the behavior for session validation.
type Auther interface {
	ResolveAuth(ctx context.Context) (*authv1.Authorization, error)
}

type AuthService struct {
	client *imauth.Client
}

func NewAuthService(client *imauth.Client) *AuthService {
	return &AuthService{client: client}
}

// ResolveAuth transparently redirects all incoming metadata to the auth service.
func (s *AuthService) ResolveAuth(ctx context.Context) (*authv1.Authorization, error) {
	// [METADATA_EXTRACTION] Capture all incoming headers
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no metadata found in context")
	}

	// [FULL_REDIRECT] Pass all original headers to the outgoing call
	// This includes X-Webitel-Access, X-Webitel-Device, X-Webitel-Client, and others.
	outCtx := metadata.NewOutgoingContext(ctx, md)

	// [IDENTITY_INSPECTION]
	// We send an empty request body because the token is already in the metadata headers.
	auth, err := s.client.Inspect(outCtx, &authv1.InspectRequest{})
	if err != nil {
		return nil, fmt.Errorf("identity inspection failed: %w", err)
	}

	return auth, nil
}
