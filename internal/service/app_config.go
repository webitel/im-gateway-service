package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

const (
	// [SUCCESS_CACHE_TTL] Admin-service toggle changes must reach new connections within this window.
	appConfigCacheTTL = 60 * time.Second
	// [FAILURE_CACHE_TTL] Transient failures are cached much shorter to avoid fail-open-caching for too long.
	appConfigFailureCacheTTL = 5 * time.Second
	// [LOOKUP_TIMEOUT] Bounds the admin-service RPC so a hung/slow backend fails open quickly instead of blocking the caller's full deadline.
	appConfigLookupTimeout = 300 * time.Millisecond
	// [NOT_FOUND_CACHE_TTL] A missing/mismatched application id is a stable negative -- cache it much longer than a transient RPC failure to avoid re-querying admin-service every 5s for an id that will never resolve.
	appConfigNotFoundCacheTTL = 5 * time.Minute
	// [CACHE_SIZE] Distinct application IDs (not users) -- expected to be small.
	appConfigCacheSize = 1024
)

// AppConfigProvider resolves per-application delivery policy for chat "system" messages.
type AppConfigProvider interface {
	// ResolvePolicy resolves the system message policy for an application within a domain.
	// Caches the result by (domainID, appID) and can be called multiple times with the same
	// (domainID, appID) pair to retrieve the cached policy. Fails open (returns allow-all policy)
	// on error. This method is primarily for use at session Attach time; once resolved and stored,
	// the policy should be reused without further RPC calls.
	ResolvePolicy(ctx context.Context, domainID int64, appID string) SystemMessagePolicy
}

// AdminAppSearcher is a narrow interface exposing only SearchApps from the admin client.
// This mirrors the ThreadStatusClient pattern: instead of importing the full *AdminClient
// concrete type into the service layer (which would make testing difficult), we define
// a minimal interface here. internal/service/di/module.go provides an adapter that
// wires *AdminClient as an AdminAppSearcher via fx.Annotate, keeping the service
// layer decoupled from the concrete client implementation.
type AdminAppSearcher interface {
	SearchApps(ctx context.Context, in *adminv1.SearchAppRequest, opts ...grpc.CallOption) (*adminv1.ApplicationList, error)
}

// SystemMessagePolicy encapsulates the decision logic for whether a system message type is allowed.
// It has three states:
//   - Zero-value (restricted=false, allowed=nil): not configured -> allow all system messages.
//   - Restricted but empty (restricted=true, allowed=[]): block all system messages.
//   - Restricted with allowed types (restricted=true, allowed=[...]): allow only listed types.
type SystemMessagePolicy struct {
	restricted bool
	allowed    []string
}

// ToDTO converts the resolved policy into the dto shape message_history.go
// forwards to im-thread-service: nil means "not restricted" (field omitted
// on the wire), non-nil (even with an empty Types slice) means "restricted".
func (p SystemMessagePolicy) ToDTO() *dto.SystemMessageAllowList {
	if !p.restricted {
		return nil
	}

	return &dto.SystemMessageAllowList{Types: p.allowed}
}

// AppConfigService implements AppConfigProvider using a cached, single-flighted admin lookup.
type AppConfigService struct {
	admin         AdminAppSearcher
	successCache  *expirable.LRU[string, SystemMessagePolicy]
	failureCache  *expirable.LRU[string, struct{}]
	notFoundCache *expirable.LRU[string, struct{}]
	singleflight  singleflight.Group
	logger        *slog.Logger
}

var _ AppConfigProvider = (*AppConfigService)(nil)

func NewAppConfigService(admin AdminAppSearcher, logger *slog.Logger) *AppConfigService {
	return &AppConfigService{
		admin:         admin,
		successCache:  expirable.NewLRU[string, SystemMessagePolicy](appConfigCacheSize, nil, appConfigCacheTTL),
		failureCache:  expirable.NewLRU[string, struct{}](appConfigCacheSize, nil, appConfigFailureCacheTTL),
		notFoundCache: expirable.NewLRU[string, struct{}](appConfigCacheSize, nil, appConfigNotFoundCacheTTL),
		logger:        logger.With("component", "app_config_service"),
	}
}

// ResolvePolicy resolves the system message policy for an application within a domain.
// Returns a zero-value (allow-all) policy immediately for empty appID to fail open
// (e.g., long-polling with no auth context). Otherwise delegates to policyFor.
func (s *AppConfigService) ResolvePolicy(ctx context.Context, domainID int64, appID string) SystemMessagePolicy {
	if appID == "" {
		return SystemMessagePolicy{}
	}

	return s.policyFor(ctx, domainID, appID)
}

// cacheKey computes a composite cache key from domainID and appID.
// Application IDs are UUIDs (guaranteed no colons), so colons are safe as separators.
func cacheKey(domainID int64, appID string) string {
	return strconv.FormatInt(domainID, 10) + ":" + appID
}

// End-user identity headers the auth interceptor (infra/auth/standard.Authorizer)
// attaches to the outgoing context. Kept as local literals (rather than importing
// infra/auth/standard) to avoid pulling a transport/auth-layer package into the
// service layer for two string constants.
const (
	xJwtPayloadHeader = "x-jwt-payload"
	xDeviceHeader     = "x-webitel-device"
)

// stripIdentityMetadata removes only the end-user identity headers (x-jwt-payload,
// x-webitel-device) from ctx's outgoing metadata, leaving any other outgoing
// metadata untouched. Used for the shared, singleflighted admin-service lookup so
// it isn't attributed to (or gated by) whichever caller happened to trigger it,
// without discarding metadata the admin-service call may legitimately need.
func stripIdentityMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return ctx
	}

	md = md.Copy()
	md.Delete(xJwtPayloadHeader)
	md.Delete(xDeviceHeader)

	return metadata.NewOutgoingContext(ctx, md)
}

// policyFor checks the success cache, then the failure cache, then the not-found cache, then calls resolve via singleflight.
// This ensures at most one concurrent RPC call per (domainID, appID) even under concurrent ResolvePolicy callers.
func (s *AppConfigService) policyFor(ctx context.Context, domainID int64, appID string) SystemMessagePolicy {
	key := cacheKey(domainID, appID)

	// Check success cache first
	if policy, ok := s.successCache.Get(key); ok {
		return policy
	}

	// Check failure cache: a transient failure means return allow-all without re-hitting RPC
	if _, ok := s.failureCache.Get(key); ok {
		return SystemMessagePolicy{}
	}

	// Check not-found cache: a definitive negative (app doesn't exist) returns allow-all much longer
	if _, ok := s.notFoundCache.Get(key); ok {
		return SystemMessagePolicy{}
	}

	// Use singleflight to call resolve exactly once even under concurrent callers
	v, _, _ := s.singleflight.Do(key, func() (any, error) {
		// Decouple the lookup from any single caller's deadline and request metadata.
		// context.WithoutCancel detaches from the triggering caller's cancellation,
		// so one caller disconnecting doesn't cancel the shared lookup for other waiters.
		// stripIdentityMetadata removes only the end-user identity headers (x-jwt-payload,
		// x-webitel-device) forwarded by the auth interceptor, so the shared, singleflighted
		// lookup isn't attributed to (or gated by) whichever caller happened to trigger it.
		// Unlike blanking the outgoing metadata wholesale, this preserves any other outgoing
		// metadata the admin-service call may need (tracing, future service credentials, etc).
		// context.WithTimeout bounds worst-case latency this hot-path call can add.
		lookupCtx := context.WithoutCancel(ctx)
		lookupCtx = stripIdentityMetadata(lookupCtx)

		lookupCtx, cancel := context.WithTimeout(lookupCtx, appConfigLookupTimeout)
		defer cancel()

		result := s.resolve(lookupCtx, domainID, appID)
		if !result.ok {
			if result.definitive {
				s.notFoundCache.Add(key, struct{}{})
			} else {
				s.failureCache.Add(key, struct{}{})
			}

			return SystemMessagePolicy{}, nil
		}

		s.successCache.Add(key, result.policy)

		return result.policy, nil
	})

	policy, ok := v.(SystemMessagePolicy)
	if !ok {
		return SystemMessagePolicy{}
	}

	return policy
}

// lookupResult carries the outcome of one admin-service SearchApps call,
// distinguishing a transient failure (network/RPC error -- may resolve
// itself soon, cached briefly) from a definitive negative (app not
// found or id mismatch -- stable, cached much longer).
type lookupResult struct {
	policy     SystemMessagePolicy
	ok         bool
	definitive bool
}

// resolve fetches the allow-list for an application from the admin service.
// Returns a lookupResult distinguishing transient failures (RPC/network errors)
// from definitive negatives (app doesn't exist or id mismatch).
//
// AppID provenance: Authorization.AppId (auth.go's AuthClient.Inspect) is the
// application under which the current user logged in. This appID is resolved
// against the same admin-service client_id.
//
// The lookup ctx passed in is already decoupled from any single caller's deadline,
// stripped of end-user forwarded identity metadata, and bounded by a fixed timeout.
// resolve itself does not need to manage these concerns -- it just uses the ctx as-is.
func (s *AppConfigService) resolve(ctx context.Context, domainID int64, appID string) lookupResult {
	res, err := s.admin.SearchApps(ctx, &adminv1.SearchAppRequest{
		Dc:     domainID,
		Id:     appID,
		Fields: []string{"id", "allow_system_messages"},
	})
	if err != nil {
		s.logger.Warn("APP_CONFIG_LOOKUP_FAILED",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID),
			slog.Any("err", err))

		return lookupResult{}
	}

	if res == nil {
		// A nil response with a nil error is a protocol anomaly (e.g. the underlying
		// RPC client returned before populating resp), not a stable "app doesn't
		// exist" -- treat it as transient so it lands in the short-TTL failureCache
		// instead of being cached as a 5-minute definitive negative.
		s.logger.Warn("APP_CONFIG_LOOKUP_FAILED",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID),
			slog.Any("err", "nil response with nil error"))

		return lookupResult{}
	}

	if len(res.GetData()) == 0 {
		s.logger.Warn("APP_CONFIG_LOOKUP_FAILED",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID),
			slog.Any("err", "no applications found"))

		return lookupResult{definitive: true}
	}

	app := res.GetData()[0]

	// [IDENTITY_CHECK] Do not blindly trust the first result: without this, a
	// permissive/relaxed Id filter on the admin-service side (or a non-unique
	// client_id) could silently apply a DIFFERENT application's policy here --
	// a cross-tenant delivery-policy decision, not just a wrong log line.
	if app.GetId() != appID {
		s.logger.Warn("APP_CONFIG_LOOKUP_MISMATCH",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID),
			slog.String("returned_id", app.GetId()))

		return lookupResult{definitive: true}
	}

	allowList := app.GetAllowSystemMessages()

	// Not configured on admin-service: allow all. This is indistinguishable here
	// from admin-service silently failing to honor the Fields mask (e.g. an
	// unrecognized field name) -- log at debug so the "no policy" case is at
	// least observable, since both cases resolve to the same fail-open policy.
	if allowList == nil {
		s.logger.Debug("APP_CONFIG_NO_POLICY",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID))

		return lookupResult{
			policy: SystemMessagePolicy{},
			ok:     true,
		}
	}

	// Configured with types: return restricted policy with allowed list
	return lookupResult{
		policy: SystemMessagePolicy{
			restricted: true,
			allowed:    allowList.GetTypes(),
		},
		ok: true,
	}
}
