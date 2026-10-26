package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
)

const (
	// [SUCCESS_CACHE_TTL] Gate / app renames must reach the right panel within this window.
	channelCacheTTL = 60 * time.Second
	// [FAILURE_CACHE_TTL] Transient lookup failures are retried soon.
	channelFailureCacheTTL = 5 * time.Second
	// [NOT_FOUND_CACHE_TTL] A deleted gate / app is a stable negative.
	channelNotFoundCacheTTL = 5 * time.Minute
	// [LOOKUP_TIMEOUT] Bounds the lookup so a slow backend only drops the channel, never the thread.
	channelLookupTimeout = 500 * time.Millisecond
	// [CACHE_SIZE] Distinct gates / apps across domains -- expected to be small.
	channelCacheSize = 1024
)

// ThreadChannelResolver resolves the channel a thread came in through.
type ThreadChannelResolver interface {
	// Resolve returns the thread channel, or nil when it cannot be resolved
	// (internal thread, missing members / contacts, lookup failure).
	Resolve(ctx context.Context, domainID int64, thr *threadv1.Thread, contacts map[string]*contact.Contact) *gtwthread.ThreadChannel
}

// GateGetter is the slice of the im-providers gate client the resolver needs.
type GateGetter interface {
	GetGate(ctx context.Context, in *providerv1.GetGateRequest, opts ...grpc.CallOption) (*providerv1.GetGateResponse, error)
}

// ThreadChannelService resolves thread channels from im-providers gates and
// im-account applications, caching both.
type ThreadChannelService struct {
	gates  GateGetter
	apps   AdminAppSearcher
	cache  *channelCache
	logger *slog.Logger
}

var _ ThreadChannelResolver = (*ThreadChannelService)(nil)

func NewThreadChannelService(gates GateGetter, apps AdminAppSearcher, logger *slog.Logger) *ThreadChannelService {
	return &ThreadChannelService{
		gates:  gates,
		apps:   apps,
		cache:  newChannelCache(),
		logger: logger.With("component", "thread_channel_service"),
	}
}

func (s *ThreadChannelService) Resolve(ctx context.Context, domainID int64, thr *threadv1.Thread, contacts map[string]*contact.Contact) *gtwthread.ThreadChannel {
	src := channelSourceOf(thr, contacts)

	switch {
	case src.gateID != "":
		return s.cache.get(ctx, "gate:"+cacheKey(domainID, src.gateID), func(ctx context.Context) (*gtwthread.ThreadChannel, error) {
			return s.gateChannel(ctx, domainID, src.gateID)
		})
	case src.appID != "":
		return s.cache.get(ctx, "app:"+cacheKey(domainID, src.appID), func(ctx context.Context) (*gtwthread.ThreadChannel, error) {
			return s.appChannel(ctx, domainID, src.appID)
		})
	default:
		return nil
	}
}

func (s *ThreadChannelService) gateChannel(ctx context.Context, domainID int64, gateID string) (*gtwthread.ThreadChannel, error) {
	res, err := s.gates.GetGate(ctx, &providerv1.GetGateRequest{Id: gateID, DomainId: domainID})
	if err != nil {
		s.logger.WarnContext(ctx, "THREAD_CHANNEL_GATE_LOOKUP_FAILED", slog.Int64("domain_id", domainID), slog.String("gate_id", gateID), slog.Any("err", err))

		return nil, err
	}

	gate := res.GetItem()

	return &gtwthread.ThreadChannel{
		Type:    gateChannelType(gate.GetType()),
		Name:    gate.GetName(),
		Address: gate.GetAddress(),
		Id:      gate.GetId(),
	}, nil
}

func (s *ThreadChannelService) appChannel(ctx context.Context, domainID int64, appID string) (*gtwthread.ThreadChannel, error) {
	res, err := s.apps.SearchApps(ctx, &adminv1.SearchAppRequest{
		Dc:     domainID,
		Id:     appID,
		Fields: []string{"id", "name", "type"},
	})
	if err != nil {
		s.logger.WarnContext(ctx, "THREAD_CHANNEL_APP_LOOKUP_FAILED", slog.Int64("domain_id", domainID), slog.String("app_id", appID), slog.Any("err", err))

		return nil, err
	}

	for _, app := range res.GetData() {
		if app.GetId() != appID {
			continue
		}

		return &gtwthread.ThreadChannel{
			Type: appChannelType(app.GetType()),
			Name: app.GetName(),
			Id:   app.GetId(),
		}, nil
	}

	return nil, status.Error(codes.NotFound, "application not found")
}

// channelSource identifies where a thread came from: the gate an external
// member reaches it through, or the application a portal client signed in via.
type channelSource struct {
	gateID string
	appID  string
}

// channelSourceOf picks the thread's channel source. A human member behind a
// gate wins; a bot-only gate is the fallback; otherwise the application of the
// first non-operator human member is used. Operators (webitel issuer) carry
// the operator app and never define the channel.
func channelSourceOf(thr *threadv1.Thread, contacts map[string]*contact.Contact) channelSource {
	var botGate string

	for _, m := range thr.GetMembers() {
		if m.GetVia() == "" {
			continue
		}

		if !m.GetIsBot() {
			return channelSource{gateID: m.GetVia()}
		}

		if botGate == "" {
			botGate = m.GetVia()
		}
	}

	if botGate != "" {
		return channelSource{gateID: botGate}
	}

	for _, m := range thr.GetMembers() {
		if m.GetIsBot() {
			continue
		}

		c := contacts[m.GetContactId()]
		if c == nil || c.GetIsBot() || c.GetIssId() == WebitelIssuer || c.GetAppId() == "" {
			continue
		}

		return channelSource{appID: c.GetAppId()}
	}

	return channelSource{}
}

// gateChannelTypes maps the provider types that have a channel logo; any other
// provider type is reported as unspecified.
var gateChannelTypes = map[providerv1.ProviderType]gtwthread.ThreadChannelType{
	providerv1.ProviderType_PROVIDER_TYPE_WHATSAPP: gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_WHATSAPP,
	providerv1.ProviderType_PROVIDER_TYPE_FACEBOOK: gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_FACEBOOK,
	providerv1.ProviderType_PROVIDER_TYPE_VIBER_BM: gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_VIBER_BM,
	providerv1.ProviderType_PROVIDER_TYPE_CUSTOM:   gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_CUSTOM,
}

func gateChannelType(t providerv1.ProviderType) gtwthread.ThreadChannelType {
	return gateChannelTypes[t] // zero value: THREAD_CHANNEL_TYPE_UNSPECIFIED
}

func appChannelType(t adminv1.AppType) gtwthread.ThreadChannelType {
	if t == adminv1.AppType_APP_TYPE_WIDGET {
		return gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_WIDGET
	}

	// Unspecified: applications created before the type existed are portals.
	return gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_PORTAL
}

// channelCache memoizes channel lookups with separate TTLs for hits, stable
// misses (not found) and transient failures, collapsing concurrent lookups of
// the same key into one RPC.
type channelCache struct {
	hits     *expirable.LRU[string, *gtwthread.ThreadChannel]
	notFound *expirable.LRU[string, struct{}]
	failures *expirable.LRU[string, struct{}]
	group    singleflight.Group
}

func newChannelCache() *channelCache {
	return &channelCache{
		hits:     expirable.NewLRU[string, *gtwthread.ThreadChannel](channelCacheSize, nil, channelCacheTTL),
		notFound: expirable.NewLRU[string, struct{}](channelCacheSize, nil, channelNotFoundCacheTTL),
		failures: expirable.NewLRU[string, struct{}](channelCacheSize, nil, channelFailureCacheTTL),
	}
}

// get returns the cached channel for key or runs lookup. A failed lookup
// yields nil. Each caller gets its own copy of the cached channel.
func (c *channelCache) get(ctx context.Context, key string, lookup func(context.Context) (*gtwthread.ThreadChannel, error)) *gtwthread.ThreadChannel {
	if ch, ok := c.hits.Get(key); ok {
		return proto.CloneOf(ch)
	}

	if c.notFound.Contains(key) || c.failures.Contains(key) {
		return nil
	}

	v, _, _ := c.group.Do(key, func() (any, error) {
		// Detach from the triggering caller (its cancellation and end-user
		// identity headers) and bound the shared lookup, as AppConfigService does.
		lookupCtx, cancel := context.WithTimeout(stripIdentityMetadata(context.WithoutCancel(ctx)), channelLookupTimeout)
		defer cancel()

		ch, err := lookup(lookupCtx)
		switch {
		case err == nil:
			c.hits.Add(key, ch)
		case status.Code(err) == codes.NotFound:
			c.notFound.Add(key, struct{}{})
		default:
			c.failures.Add(key, struct{}{})
		}

		return ch, nil
	})

	ch, ok := v.(*gtwthread.ThreadChannel)
	if !ok {
		return nil
	}

	return proto.CloneOf(ch)
}
