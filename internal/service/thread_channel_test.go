package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

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

type fakeGates struct {
	calls atomic.Int32
	fn    func(in *providerv1.GetGateRequest) (*providerv1.GetGateResponse, error)
}

func (f *fakeGates) GetGate(_ context.Context, in *providerv1.GetGateRequest, _ ...grpc.CallOption) (*providerv1.GetGateResponse, error) {
	f.calls.Add(1)

	return f.fn(in)
}

type fakeApps struct {
	calls atomic.Int32
	fn    func(in *adminv1.SearchAppRequest) (*adminv1.ApplicationList, error)
}

func (f *fakeApps) SearchApps(_ context.Context, in *adminv1.SearchAppRequest, _ ...grpc.CallOption) (*adminv1.ApplicationList, error) {
	f.calls.Add(1)

	return f.fn(in)
}

func TestChannelSourceOf(t *testing.T) {
	contacts := map[string]*contact.Contact{
		"client":   {Id: "client", IssId: "https://portal.example", AppId: "app-1"},
		"operator": {Id: "operator", IssId: WebitelIssuer, AppId: "app-operator"},
		"no-app":   {Id: "no-app", IssId: "https://portal.example"},
	}

	tests := []struct {
		name    string
		members []*threadv1.ThreadMember
		want    channelSource
	}{
		{
			name: "external human member wins over bot gate and app",
			members: []*threadv1.ThreadMember{
				{ContactId: "bot", IsBot: true, Via: "gate-bot"},
				{ContactId: "client"},
				{ContactId: "wa-client", Via: "gate-1"},
			},
			want: channelSource{gateID: "gate-1"},
		},
		{
			name: "bot-only gate is the fallback",
			members: []*threadv1.ThreadMember{
				{ContactId: "operator"},
				{ContactId: "bot", IsBot: true, Via: "gate-bot"},
			},
			want: channelSource{gateID: "gate-bot"},
		},
		{
			name: "portal client application",
			members: []*threadv1.ThreadMember{
				{ContactId: "operator"},
				{ContactId: "client"},
			},
			want: channelSource{appID: "app-1"},
		},
		{
			name: "operators never define the channel",
			members: []*threadv1.ThreadMember{
				{ContactId: "operator"},
			},
			want: channelSource{},
		},
		{
			name: "contact without application or unknown contact",
			members: []*threadv1.ThreadMember{
				{ContactId: "no-app"},
				{ContactId: "missing"},
			},
			want: channelSource{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := channelSourceOf(&threadv1.Thread{Members: tt.members}, contacts)
			if got != tt.want {
				t.Errorf("channelSourceOf() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestThreadChannelService_Gate(t *testing.T) {
	tests := []struct {
		providerType providerv1.ProviderType
		want         gtwthread.ThreadChannelType
	}{
		{providerv1.ProviderType_PROVIDER_TYPE_WHATSAPP, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_WHATSAPP},
		{providerv1.ProviderType_PROVIDER_TYPE_FACEBOOK, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_FACEBOOK},
		{providerv1.ProviderType_PROVIDER_TYPE_VIBER_BM, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_VIBER_BM},
		{providerv1.ProviderType_PROVIDER_TYPE_CUSTOM, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_CUSTOM},
		{providerv1.ProviderType_PROVIDER_TYPE_TELEGRAM_BOT, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.providerType.String(), func(t *testing.T) {
			gates := &fakeGates{fn: func(in *providerv1.GetGateRequest) (*providerv1.GetGateResponse, error) {
				if in.GetId() != "gate-1" || in.GetDomainId() != 7 {
					t.Errorf("GetGate(%q, %d), want (gate-1, 7)", in.GetId(), in.GetDomainId())
				}

				return &providerv1.GetGateResponse{Item: &providerv1.ProviderSummary{
					Id: "gate-1", Name: "Shop", Type: tt.providerType, Address: "+380671234567",
				}}, nil
			}}
			svc := NewThreadChannelService(gates, &fakeApps{}, discardLogger())

			thr := &threadv1.Thread{Members: []*threadv1.ThreadMember{{ContactId: "c", Via: "gate-1"}}}
			got := svc.Resolve(context.Background(), 7, thr, nil)

			want := &gtwthread.ThreadChannel{Type: tt.want, Name: "Shop", Address: "+380671234567", Id: "gate-1"}
			if !proto.Equal(got, want) {
				t.Errorf("Resolve() = %v, want %v", got, want)
			}
		})
	}
}

func TestThreadChannelService_App(t *testing.T) {
	tests := []struct {
		name    string
		appType adminv1.AppType
		want    gtwthread.ThreadChannelType
	}{
		{"portal", adminv1.AppType_APP_TYPE_PORTAL, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_PORTAL},
		{"widget", adminv1.AppType_APP_TYPE_WIDGET, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_WIDGET},
		{"legacy app without type", adminv1.AppType_APP_TYPE_UNSPECIFIED, gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_PORTAL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps := &fakeApps{fn: func(in *adminv1.SearchAppRequest) (*adminv1.ApplicationList, error) {
				if in.GetId() != "app-1" || in.GetDc() != 7 {
					t.Errorf("SearchApps(%q, %d), want (app-1, 7)", in.GetId(), in.GetDc())
				}

				return &adminv1.ApplicationList{Data: []*adminv1.Application{{Id: "app-1", Name: "Portal", Type: tt.appType}}}, nil
			}}
			svc := NewThreadChannelService(&fakeGates{}, apps, discardLogger())

			thr := &threadv1.Thread{Members: []*threadv1.ThreadMember{{ContactId: "client"}}}
			contacts := map[string]*contact.Contact{"client": {Id: "client", IssId: "https://portal.example", AppId: "app-1"}}
			got := svc.Resolve(context.Background(), 7, thr, contacts)

			want := &gtwthread.ThreadChannel{Type: tt.want, Name: "Portal", Id: "app-1"}
			if !proto.Equal(got, want) {
				t.Errorf("Resolve() = %v, want %v", got, want)
			}
		})
	}
}

func TestThreadChannelService_CachesAndFailsSoft(t *testing.T) {
	thr := &threadv1.Thread{Members: []*threadv1.ThreadMember{{ContactId: "c", Via: "gate-1"}}}

	t.Run("hit is cached and copied", func(t *testing.T) {
		gates := &fakeGates{fn: func(*providerv1.GetGateRequest) (*providerv1.GetGateResponse, error) {
			return &providerv1.GetGateResponse{Item: &providerv1.ProviderSummary{Id: "gate-1", Name: "Shop"}}, nil
		}}
		svc := NewThreadChannelService(gates, &fakeApps{}, discardLogger())

		first := svc.Resolve(context.Background(), 1, thr, nil)
		first.Name = "mutated"
		second := svc.Resolve(context.Background(), 1, thr, nil)

		if gates.calls.Load() != 1 {
			t.Errorf("GetGate calls = %d, want 1", gates.calls.Load())
		}

		if second.GetName() != "Shop" {
			t.Errorf("cached channel was mutated through a returned copy: %q", second.GetName())
		}
	})

	t.Run("cache is per domain", func(t *testing.T) {
		gates := &fakeGates{fn: func(*providerv1.GetGateRequest) (*providerv1.GetGateResponse, error) {
			return &providerv1.GetGateResponse{Item: &providerv1.ProviderSummary{Id: "gate-1"}}, nil
		}}
		svc := NewThreadChannelService(gates, &fakeApps{}, discardLogger())

		svc.Resolve(context.Background(), 1, thr, nil)
		svc.Resolve(context.Background(), 2, thr, nil)

		if gates.calls.Load() != 2 {
			t.Errorf("GetGate calls = %d, want 2", gates.calls.Load())
		}
	})

	for _, tt := range []struct {
		name string
		err  error
	}{
		{"not found", status.Error(codes.NotFound, "gate not found")},
		{"transient failure", errors.New("connection refused")},
	} {
		t.Run(tt.name+" yields no channel and is cached", func(t *testing.T) {
			gates := &fakeGates{fn: func(*providerv1.GetGateRequest) (*providerv1.GetGateResponse, error) {
				return nil, tt.err
			}}
			svc := NewThreadChannelService(gates, &fakeApps{}, discardLogger())

			for range 2 {
				if got := svc.Resolve(context.Background(), 1, thr, nil); got != nil {
					t.Errorf("Resolve() = %v, want nil", got)
				}
			}

			if gates.calls.Load() != 1 {
				t.Errorf("GetGate calls = %d, want 1", gates.calls.Load())
			}
		})
	}

	t.Run("app missing from the result", func(t *testing.T) {
		apps := &fakeApps{fn: func(*adminv1.SearchAppRequest) (*adminv1.ApplicationList, error) {
			return &adminv1.ApplicationList{}, nil
		}}
		svc := NewThreadChannelService(&fakeGates{}, apps, discardLogger())

		thr := &threadv1.Thread{Members: []*threadv1.ThreadMember{{ContactId: "client"}}}

		contacts := map[string]*contact.Contact{"client": {Id: "client", IssId: "https://portal.example", AppId: "app-1"}}
		if got := svc.Resolve(context.Background(), 1, thr, contacts); got != nil {
			t.Errorf("Resolve() = %v, want nil", got)
		}
	})

	t.Run("internal thread does not call out", func(t *testing.T) {
		gates, apps := &fakeGates{}, &fakeApps{}
		svc := NewThreadChannelService(gates, apps, discardLogger())

		if got := svc.Resolve(context.Background(), 1, &threadv1.Thread{}, nil); got != nil {
			t.Errorf("Resolve() = %v, want nil", got)
		}

		if gates.calls.Load()+apps.calls.Load() != 0 {
			t.Error("unexpected lookup for a thread without a channel source")
		}
	})
}
