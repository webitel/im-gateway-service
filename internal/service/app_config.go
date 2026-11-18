package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

const (
	appConfigCacheTTL         = 60 * time.Second
	appConfigFailureCacheTTL  = 5 * time.Second
	appConfigLookupTimeout    = 300 * time.Millisecond
	appConfigNotFoundCacheTTL = appConfigCacheTTL
	appConfigCacheSize        = 1024
)

type AppConfigProvider interface {
	ResolveAppPolicy(ctx context.Context, domainID int64, appID string) (AppPolicy, error)
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

// ContactPolicy mirrors the presence semantics of admin allow_contacts:
//   - zero value: allow_contacts unset -> no restriction;
//   - restricted with an empty set: nobody may be addressed.
type ContactPolicy struct {
	restricted bool
	allowed    map[string]struct{}
	defaultID  string
}

func NewContactPolicy(list *adminv1.ContactAllowList) ContactPolicy {
	if list == nil {
		return ContactPolicy{}
	}

	policy := ContactPolicy{
		restricted: true,
		allowed:    make(map[string]struct{}, len(list.GetRules())),
	}

	for _, rule := range list.GetRules() {
		contactID := rule.GetContactId()
		if contactID == "" {
			continue
		}

		policy.allowed[contactID] = struct{}{}

		if rule.GetDefault() && policy.defaultID == "" {
			policy.defaultID = contactID
		}
	}

	return policy
}

// NewAppContactPolicy: a WIDGET app without allow_contacts is deny-all, because its client id is
// public and must not reach arbitrary contacts until an admin configures the list explicitly.
func NewAppContactPolicy(app *adminv1.Application) ContactPolicy {
	list := app.GetAllowContacts()
	if list == nil && app.GetType() == adminv1.AppType_APP_TYPE_WIDGET {
		list = &adminv1.ContactAllowList{}
	}

	return NewContactPolicy(list)
}

func (p ContactPolicy) Restricted() bool {
	return p.restricted
}

func (p ContactPolicy) Allows(contactID string) bool {
	if !p.restricted {
		return true
	}

	_, ok := p.allowed[contactID]

	return ok
}

// AllowedIDs narrows requested contact ids to the allow-list; no requested ids means the whole list.
// Only meaningful for a restricted policy.
func (p ContactPolicy) AllowedIDs(requested []string) []string {
	if len(requested) == 0 {
		return slices.Collect(maps.Keys(p.allowed))
	}

	return slices.DeleteFunc(slices.Clone(requested), func(id string) bool {
		_, ok := p.allowed[id]

		return !ok
	})
}

func (p ContactPolicy) DefaultContactID() (string, bool) {
	return p.defaultID, p.defaultID != ""
}

// resolveContactPolicy: callers without an AppID (operators, schema, engine) are unrestricted.
func resolveContactPolicy(ctx context.Context, provider AppConfigProvider, session auth.Identifier) (ContactPolicy, error) {
	if provider == nil || session.GetApplicationID() == "" {
		return ContactPolicy{}, nil
	}

	policy, err := provider.ResolveAppPolicy(ctx, session.GetDomainID(), session.GetApplicationID())
	if err != nil {
		return ContactPolicy{}, err
	}

	return policy.Contacts, nil
}

// checkContactAllowed: an empty contactID means the contact does not exist. It is denied exactly like
// a contact outside the allow-list, so a restricted (public) client cannot probe which identities
// exist in the domain.
func checkContactAllowed(policy ContactPolicy, contactID string) error {
	if contactID == "" || !policy.Allows(contactID) {
		return contactNotAllowedErr()
	}

	return nil
}

func contactNotAllowedErr() error {
	return errors.Forbidden("contact is not allowed for this application", errors.WithID("service.contact_policy.not_allowed"))
}

type AppPolicy struct {
	SystemMessages SystemMessagePolicy
	Contacts       ContactPolicy
}

// AppConfigService implements AppConfigProvider using a cached, single-flighted admin lookup.
type AppConfigService struct {
	admin         AdminAppSearcher
	successCache  *expirable.LRU[string, AppPolicy]
	failureCache  *expirable.LRU[string, struct{}]
	notFoundCache *expirable.LRU[string, struct{}]
	singleflight  singleflight.Group
	logger        *slog.Logger
}

var _ AppConfigProvider = (*AppConfigService)(nil)

func NewAppConfigService(admin AdminAppSearcher, logger *slog.Logger) *AppConfigService {
	return &AppConfigService{
		admin:         admin,
		successCache:  expirable.NewLRU[string, AppPolicy](appConfigCacheSize, nil, appConfigCacheTTL),
		failureCache:  expirable.NewLRU[string, struct{}](appConfigCacheSize, nil, appConfigFailureCacheTTL),
		notFoundCache: expirable.NewLRU[string, struct{}](appConfigCacheSize, nil, appConfigNotFoundCacheTTL),
		logger:        logger.With("component", "app_config_service"),
	}
}

func (s *AppConfigService) ResolveAppPolicy(ctx context.Context, domainID int64, appID string) (AppPolicy, error) {
	if appID == "" {
		return AppPolicy{}, nil
	}

	return s.appPolicyFor(ctx, domainID, appID)
}

// A fresh error per call: callers never share mutable error state.
func appConfigUnavailableErr() error {
	return errors.Unavailable("application policy is temporarily unavailable", errors.WithID("service.app_config.resolve.unavailable"))
}

func appNotFoundErr() error {
	return errors.Forbidden("application not found", errors.WithID("service.app_config.resolve.app_not_found"))
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

func (s *AppConfigService) appPolicyFor(ctx context.Context, domainID int64, appID string) (AppPolicy, error) {
	key := cacheKey(domainID, appID)

	// Check success cache first
	if policy, ok := s.successCache.Get(key); ok {
		return policy, nil
	}

	// Short back-off: a cached transient failure is an error without re-hitting the RPC.
	if _, ok := s.failureCache.Get(key); ok {
		return AppPolicy{}, appConfigUnavailableErr()
	}

	// A definitive negative (app does not exist) is cached longer than a transient failure.
	if _, ok := s.notFoundCache.Get(key); ok {
		return AppPolicy{}, appNotFoundErr()
	}

	// Use singleflight to call resolve exactly once even under concurrent callers
	v, _, _ := s.singleflight.Do(key, func() (any, error) {
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

			return result, nil
		}

		s.successCache.Add(key, result.policy)

		return result, nil
	})

	// Every singleflight waiter shares the same lookupResult, so all of them map it identically.
	result, ok := v.(lookupResult)
	if !ok {
		return AppPolicy{}, appConfigUnavailableErr()
	}

	switch {
	case result.ok:
		return result.policy, nil
	case result.definitive:
		return AppPolicy{}, appNotFoundErr()
	default:
		return AppPolicy{}, appConfigUnavailableErr()
	}
}

// lookupResult carries the outcome of one admin-service SearchApps call,
// distinguishing a transient failure (network/RPC error -- may resolve
// itself soon, cached briefly) from a definitive negative (app not
// found or id mismatch -- stable, cached much longer).
type lookupResult struct {
	policy     AppPolicy
	ok         bool
	definitive bool
}

// resolve fetches the allow-lists for an application from the admin service.
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
		Fields: []string{"id", "type", "allow_system_messages", "allow_contacts"},
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
		// instead of being cached as a definitive negative (appConfigNotFoundCacheTTL).
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

	policy := AppPolicy{Contacts: NewAppContactPolicy(app)}

	if allowList := app.GetAllowSystemMessages(); allowList != nil {
		// Configured with types: return restricted policy with allowed list
		policy.SystemMessages = SystemMessagePolicy{
			restricted: true,
			allowed:    allowList.GetTypes(),
		}
	}

	// Not configured on admin-service: allow all. This is indistinguishable here
	// from admin-service silently failing to honor the Fields mask (e.g. an
	// unrecognized field name) -- log at debug so the "no policy" case is at
	// least observable, since both cases resolve to the same fail-open policy.
	if app.GetAllowSystemMessages() == nil && !policy.Contacts.Restricted() {
		s.logger.Debug("APP_CONFIG_NO_POLICY",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID))
	}

	if policy.Contacts.Restricted() {
		_, hasDefault := policy.Contacts.DefaultContactID()

		s.logger.Debug("APP_CONFIG_CONTACTS_POLICY",
			slog.Int64("domain_id", domainID),
			slog.String("app_id", appID),
			slog.Bool("restricted", true),
			slog.Int("rules", len(app.GetAllowContacts().GetRules())),
			slog.Bool("has_default", hasDefault))
	}

	return lookupResult{
		policy: policy,
		ok:     true,
	}
}
