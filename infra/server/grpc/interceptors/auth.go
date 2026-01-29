package interceptors

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	"github.com/webitel/im-gateway-service/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	// AuthContextKey is used to store/retrieve Identity from context
	AuthContextKey contextKey = "auth_identity"

	// WorkflowIssuerCN is the expected Issuer Common Name for production workflow certificates
	WorkflowIssuerCN = "workflow"

	// DevIssuerCN is used for local development purposes (e.g., self-signed certificates)
	DevIssuerCN = "My CA"
)

// Identity stores the resolved domain ownership and contact ID
type Identity struct {
	ContactID string
	DomainID  int64
}

// NewUnaryAuthInterceptor provides identification for standard RPC calls
func NewUnaryAuthInterceptor(auther service.Auther, contacter imcontact.Client, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// [RESOLVE] Execute identification logic using both auther (sessions) and contacter (mTLS)
		id, err := resolveIdentity(ctx, auther, contacter, logger)
		if err != nil {
			return nil, err
		}

		// [ENRICHMENT] Inject the identity into the context for downstream handlers
		newCtx := context.WithValue(ctx, AuthContextKey, id)

		return handler(newCtx, req)
	}
}

// resolveIdentity determines identification path based on connection type and headers
func resolveIdentity(ctx context.Context, auther service.Auther, contacter imcontact.Client, logger *slog.Logger) (*Identity, error) {
	// [MTLS_DETECTION] Validate peer connection and check Issuer
	ismTLS := false
	if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok && len(tlsInfo.State.PeerCertificates) > 0 {
			// Get the leaf certificate
			leaf := tlsInfo.State.PeerCertificates[0]
			// [ISSUER_CHECK] Verify that the certificate was issued by "workflow"
			// This prevents spoofing using certificates from other internal CAs
			issuer := leaf.Issuer.CommonName
			if issuer == WorkflowIssuerCN || issuer == DevIssuerCN {
				ismTLS = true
			} else {
				logger.Debug("mTLS certificate detected but issuer mismatch",
					slog.String("issuer", issuer),
					slog.String("subject", leaf.Subject.CommonName),
				)
			}
		}
	}

	// [METADATA_EXTRACT] FETCH incoming gRPC context headers
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "AUTHENTICATION_REQUIRED: metadata is missing")
	}

	if ismTLS {
		authType := getHeader(md, "x-webitel-type")

		if authType == "schema" {
			// [PATH: SCHEMA_AUTH] EXPECTED HEADER: "x-webitel-schema" FORMAT: "{domainID}.{subject}"
			rawSchema := getHeader(md, "x-webitel-schema")
			if rawSchema == "" {
				return nil, status.Error(codes.Unauthenticated, "SCHEMA_AUTH_FAILED: x-webitel-schema header is required for mTLS schema type")
			}

			// [PARSE] EXTRACT domain and subject
			domainID, sub := splitDomainAndSub(rawSchema, logger)

			// [GUARD] ENFORCE strict presence of required identity components
			if domainID == 0 || sub == "" {
				return nil, status.Error(codes.Unauthenticated, "IDENTITY_INCOMPLETE: missing domainID or subject in x-webitel-schema")
			}

			// [RESOLVE_CONTACT] SEARCH for existing identity in the Contact Service
			res, err := contacter.SearchContact(ctx, &contactv1.SearchContactRequest{
				Subjects: []string{sub},
				DomainId: int32(domainID),
				Size:     1,
			})

			// [VALIDATE_RECORD] ENSURE the resolved entity exists
			if err != nil || len(res.GetContacts()) == 0 {
				logger.Warn("IDENTITY_NOT_FOUND",
					slog.String("sub", sub),
					slog.Int64("domain", domainID),
					slog.Any("error", err),
				)
				return nil, status.Error(codes.NotFound, "IDENTITY_NOT_REGISTERED: contact mapping failed")
			}

			return &Identity{
				ContactID: res.GetContacts()[0].Id,
				DomainID:  domainID,
			}, nil
		}
	}

	// [PATH: SESSION_INSPECT] FALLBACK to token-based or session-based verification
	auth, err := auther.Inspect(ctx)
	if err != nil {
		logger.Error("INSPECT_FAILURE", slog.Any("err", err))
		return nil, err
	}

	return &Identity{
		ContactID: auth.ContactID,
		DomainID:  auth.DC,
	}, nil
}

// GetIdentity is a helper to extract the identity from context safely.
func GetIdentity(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(AuthContextKey).(*Identity)
	return id, ok
}

// --- Internal Helpers ---

func splitDomainAndSub(raw string, logger *slog.Logger) (int64, string) {
	if raw == "" {
		return 0, ""
	}
	parts := strings.SplitN(raw, ".", 2)
	domainID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		logger.Warn("failed to parse domain part", slog.String("raw", raw))
		return 0, ""
	}
	if len(parts) < 2 {
		return domainID, ""
	}
	return domainID, parts[1]
}

func getHeader(md metadata.MD, key string) string {
	if vals := md.Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}
