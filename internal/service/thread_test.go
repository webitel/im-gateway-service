package service

import (
	"context"
	"log/slog"
	"math"
	"reflect"
	"slices"
	"testing"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/model"
)

var contactData = map[string]*contactv1.Contact{
	"1": {
		Id:       "1",
		Name:     "Contact One",
		Subject:  "1",
		IssId:    "1",
		Username: "one",
	},
	"2": {
		Id:       "2",
		Name:     "Contact Two",
		IssId:    "1",
		Subject:  "2",
		Username: "two",
	},
	"3": {
		Id:       "3",
		Name:     "Contact Three",
		IssId:    "1",
		Subject:  "3",
		Username: "three",
	},
}

func equalThreadEntities(got, want []*threadv1.Entity) bool {
	if (got == nil) != (want == nil) {
		return false
	}

	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].GetType() != want[i].GetType() {
			return false
		}

		if got[i].GetOffset() != want[i].GetOffset() {
			return false
		}

		if got[i].GetLength() != want[i].GetLength() {
			return false
		}

		if (got[i].Value == nil) != (want[i].Value == nil) {
			return false
		}

		if got[i].Value != nil && *got[i].Value != *want[i].Value {
			return false
		}
	}

	return true
}

func Test_convertToThread(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		thr         *threadv1.Thread
		contactData map[string]*contactv1.Contact
		want        *gtwthread.Thread
	}{
		{
			name: "Test with a simple thread and valid contact data",
			thr: &threadv1.Thread{
				Id:          "thread1",
				Kind:        threadv1.ThreadKind_DIRECT,
				Subject:     "thread1",
				Description: "This is thread one",
				Members: []*threadv1.ThreadMember{
					{
						Id:        "member1",
						Role:      threadv1.ThreadRole_ROLE_OWNER,
						ContactId: "1",
					},
					{
						Id:        "member2",
						Role:      threadv1.ThreadRole_ROLE_OWNER,
						ContactId: "2",
					},
				},
			},
			contactData: contactData,
			want: &gtwthread.Thread{
				Id:          "thread1",
				Subject:     "thread1",
				Type:        gtwthread.ThreadKind_DIRECT,
				Description: "This is thread one",
				Members: []*gtwthread.ThreadMember{
					{
						Id:   "member1",
						Role: gtwthread.ThreadRole_ROLE_OWNER,
						Contact: &gtwthread.Contact{
							Name:     "Contact One",
							Id:       "1",
							Iss:      "1",
							Sub:      "1",
							Username: "one",
						},
					},
					{
						Id:   "member2",
						Role: gtwthread.ThreadRole_ROLE_OWNER,
						Contact: &gtwthread.Contact{
							Name:     "Contact Two",
							Id:       "2",
							Iss:      "1",
							Sub:      "2",
							Username: "two",
						},
					},
				},
			},
		},
		{
			name: "Test convertToThread maps crm_contact_id",
			thr: &threadv1.Thread{
				Id:           "thread2",
				Kind:         threadv1.ThreadKind_DIRECT,
				Subject:      "thread2",
				CrmContactId: "crm-123",
				Description:  "Thread with CRM contact",
				Members:      []*threadv1.ThreadMember{},
			},
			contactData: contactData,
			want: &gtwthread.Thread{
				Id:           "thread2",
				Subject:      "thread2",
				Type:         gtwthread.ThreadKind_DIRECT,
				Description:  "Thread with CRM contact",
				CrmContactId: "crm-123",
				Members:      []*gtwthread.ThreadMember{},
			},
		},
		{
			name: "Test convertToThread maps the caller's tags",
			thr: &threadv1.Thread{
				Id:      "thread3",
				Kind:    threadv1.ThreadKind_GROUP,
				Members: []*threadv1.ThreadMember{},
				Tags: []*threadv1.ChatTag{
					{Id: "tag1", ThreadId: "thread3", ContactId: "1", Tag: "snoozed", CreatedAt: 1700},
				},
			},
			contactData: contactData,
			want: &gtwthread.Thread{
				Id:      "thread3",
				Type:    gtwthread.ThreadKind_GROUP,
				Members: []*gtwthread.ThreadMember{},
				Tags: []*gtwthread.ChatTag{
					{Id: "tag1", ThreadId: "thread3", Tag: "snoozed", CreatedAt: 1700},
				},
			},
		},
		{
			name: "queue is mapped when set",
			thr: &threadv1.Thread{
				Id:      "thread4",
				Kind:    threadv1.ThreadKind_DIRECT,
				Members: []*threadv1.ThreadMember{},
				Source:  &threadv1.ThreadSource{Id: "gate-1", Name: "Support", Type: "facebook"},
				Queue:   &threadv1.ThreadQueue{Id: 7, Name: "Sales"},
			},
			want: &gtwthread.Thread{
				Id:      "thread4",
				Type:    gtwthread.ThreadKind_DIRECT,
				Members: []*gtwthread.ThreadMember{},
				Queue:   &gtwthread.ThreadQueue{Id: 7, Name: "Sales"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToThread(tt.thr, tt.contactData)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToThread() = got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_toThreadEntities(t *testing.T) {
	url1 := "https://example.com"
	url2 := "https://example.com/page"

	tests := []struct {
		name     string
		entities []model.Entity
		want     []*threadv1.Entity
	}{
		{
			name:     "empty entities slice returns nil",
			entities: []model.Entity{},
			want:     nil,
		},
		{
			name:     "nil entities slice returns nil",
			entities: nil,
			want:     nil,
		},
		{
			name: "single BOLD entity",
			entities: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
			},
			want: []*threadv1.Entity{
				{
					Type:   string(model.EntityTypeBold),
					Offset: 0,
					Length: 4,
					Value:  nil,
				},
			},
		},
		{
			name: "multiple entities with LINK having a value",
			entities: []model.Entity{
				{
					Type:   model.EntityTypeBold,
					Offset: 0,
					Length: 5,
					Value:  nil,
				},
				{
					Type:   model.EntityTypeLink,
					Offset: 6,
					Length: 4,
					Value:  &url1,
				},
				{
					Type:   model.EntityTypeItalic,
					Offset: 11,
					Length: 6,
					Value:  nil,
				},
			},
			want: []*threadv1.Entity{
				{
					Type:   string(model.EntityTypeBold),
					Offset: 0,
					Length: 5,
					Value:  nil,
				},
				{
					Type:   string(model.EntityTypeLink),
					Offset: 6,
					Length: 4,
					Value:  &url1,
				},
				{
					Type:   string(model.EntityTypeItalic),
					Offset: 11,
					Length: 6,
					Value:  nil,
				},
			},
		},
		{
			name: "all entity types",
			entities: []model.Entity{
				{Type: model.EntityTypeBold, Offset: 0, Length: 1, Value: nil},
				{Type: model.EntityTypeItalic, Offset: 1, Length: 1, Value: nil},
				{Type: model.EntityTypeStrikethrough, Offset: 2, Length: 1, Value: nil},
				{Type: model.EntityTypeCode, Offset: 3, Length: 1, Value: nil},
				{Type: model.EntityTypePre, Offset: 4, Length: 1, Value: nil},
				{Type: model.EntityTypeLink, Offset: 5, Length: 1, Value: &url2},
			},
			want: []*threadv1.Entity{
				{Type: string(model.EntityTypeBold), Offset: 0, Length: 1, Value: nil},
				{Type: string(model.EntityTypeItalic), Offset: 1, Length: 1, Value: nil},
				{Type: string(model.EntityTypeStrikethrough), Offset: 2, Length: 1, Value: nil},
				{Type: string(model.EntityTypeCode), Offset: 3, Length: 1, Value: nil},
				{Type: string(model.EntityTypePre), Offset: 4, Length: 1, Value: nil},
				{Type: string(model.EntityTypeLink), Offset: 5, Length: 1, Value: &url2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toThreadEntities(tt.entities)
			if !equalThreadEntities(got, tt.want) {
				t.Errorf("toThreadEntities() = got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_thread_Search(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		logger        *slog.Logger
		threadClient  *imthread.ThreadClient
		contactClient *imcontact.Client
		// Named input parameters for target function.
		searchQuery *gtwthread.ThreadSearchRequest
		want        *gtwthread.SearchThreadResponse
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := NewThread(tt.logger, tt.threadClient, tt.contactClient, nil)

			got, gotErr := th.Search(context.Background(), tt.searchQuery)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Search() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Search() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("Search() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_thread_collectUniqueContactsFromThread(t *testing.T) {
	tests := []struct {
		name    string
		threads []*threadv1.Thread
		want    []string
	}{
		{
			name: "collects unique ids from members last message and variables",
			threads: []*threadv1.Thread{
				{
					Members: []*threadv1.ThreadMember{
						{ContactId: "contact-1"},
						{ContactId: "contact-2"},
						{ContactId: "contact-1"},
					},
					LastMsg: &threadv1.HistoryMessage{SenderId: "contact-3"},
					Variables: &threadv1.ThreadVariables{
						Variables: map[string]*threadv1.VariableEntry{
							"a": {SetBy: "contact-4"},
							"b": {SetBy: "contact-2"},
						},
					},
				},
				{
					Members: []*threadv1.ThreadMember{
						{ContactId: "contact-5"},
					},
					LastMsg: &threadv1.HistoryMessage{SenderId: "contact-4"},
				},
			},
			want: []string{"contact-1", "contact-2", "contact-3", "contact-4", "contact-5"},
		},
		{
			name: "does not include empty ids",
			threads: []*threadv1.Thread{
				{
					Members: []*threadv1.ThreadMember{
						{ContactId: ""},
						{ContactId: "contact-1"},
					},
					LastMsg: &threadv1.HistoryMessage{SenderId: ""},
					Variables: &threadv1.ThreadVariables{
						Variables: map[string]*threadv1.VariableEntry{
							"empty":  {SetBy: ""},
							"valid":  {SetBy: "contact-2"},
							"repeat": {SetBy: "contact-1"},
						},
					},
				},
			},
			want: []string{"contact-1", "contact-2"},
		},
	}

	th := &thread{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.collectUniqueContactsFromThread(tt.threads)

			slices.Sort(got)
			slices.Sort(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectUniqueContactsFromThread() = %v, want %v", got, tt.want)
			}
		})
	}
}

type readCallerIdentity struct {
	contactID string
}

func (i readCallerIdentity) GetContactID() string     { return i.contactID }
func (i readCallerIdentity) GetDomainID() int64       { return 1 }
func (i readCallerIdentity) GetIssuer() string        { return "" }
func (i readCallerIdentity) GetName() string          { return "" }
func (i readCallerIdentity) GetChatName() string      { return "" }
func (i readCallerIdentity) GetApplicationID() string { return "" }
func (i readCallerIdentity) GetVia() string           { return "" }
func (i readCallerIdentity) GetViaPtr() *string       { return nil }
func (i readCallerIdentity) GetViaName() string       { return "" }
func (i readCallerIdentity) GetViaType() string       { return "" }

func Test_readCallerID(t *testing.T) {
	identity := readCallerIdentity{contactID: "contact-1"}

	tests := []struct {
		name     string
		authType auth.XWebitelType
		hasType  bool
		want     string
	}{
		{name: "regular user gets own contact id", want: "contact-1"},
		{name: "schema system call gets empty caller", authType: auth.XWebitelTypeSchema, hasType: true, want: ""},
		{name: "engine system call gets empty caller", authType: auth.XWebitelTypeEngine, hasType: true, want: ""},
		{name: "provider call keeps caller id", authType: auth.XWebitelTypeProvider, hasType: true, want: "contact-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.hasType {
				ctx = auth.WithAuthType(ctx, tt.authType)
			}

			if got := readCallerID(ctx, identity); got != tt.want {
				t.Errorf("readCallerID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetCrmContactValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     *gtwthread.SetCrmContactRequest
		wantErr bool
	}{
		{
			name:    "missing thread id returns error",
			req:     &gtwthread.SetCrmContactRequest{ThreadId: ""},
			wantErr: true,
		},
		{
			name:    "missing identity in context returns error",
			req:     &gtwthread.SetCrmContactRequest{ThreadId: "thread1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &thread{
				logger:       slog.New(slog.NewTextHandler(nil, nil)),
				threadClient: nil,
			}

			resp, err := th.SetCrmContact(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetCrmContact() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && resp != nil {
				t.Errorf("SetCrmContact() expected nil response on error, got %v", resp)
			}
		})
	}
}

// Members carry the IM contact id so the client can look the contact up in the CRM.
func Test_convertToContact_ID(t *testing.T) {
	got := convertToContact(&contactv1.Contact{Id: "0190a6f6-0000-7000-8000-000000000001", IssId: "facebook", Subject: "psid-1", Name: "Ivan"})

	if got.GetId() != "0190a6f6-0000-7000-8000-000000000001" {
		t.Errorf("convertToContact().Id = %q, want contact id", got.GetId())
	}
}

func Test_queueFromContext(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	tests := []struct {
		name string
		md   metadata.MD
		want *threadv1.ThreadQueue
	}{
		{name: "no metadata is no queue", md: nil, want: nil},
		{name: "no id header is no queue", md: metadata.Pairs(auth.QueueNameIdentificationHeader, "Sales"), want: nil},
		{name: "zero id is no queue", md: metadata.Pairs(auth.QueueIDIdentificationHeader, "0"), want: nil},
		{name: "negative id is no queue", md: metadata.Pairs(auth.QueueIDIdentificationHeader, "-3"), want: nil},
		{name: "non-numeric id is no queue", md: metadata.Pairs(auth.QueueIDIdentificationHeader, "abc"), want: nil},
		{name: "id only", md: metadata.Pairs(auth.QueueIDIdentificationHeader, "7"), want: &threadv1.ThreadQueue{Id: 7}},
		{
			name: "id and non-ASCII name",
			md:   metadata.Pairs(auth.QueueIDIdentificationHeader, "7", auth.QueueNameIdentificationHeader, "Продажі"),
			want: &threadv1.ThreadQueue{Id: 7, Name: "Продажі"},
		},
		{
			name: "invalid UTF-8 name is coerced",
			md:   metadata.Pairs(auth.QueueIDIdentificationHeader, "7", auth.QueueNameIdentificationHeader, "a\xffb"),
			want: &threadv1.ThreadQueue{Id: 7, Name: "a\uFFFDb"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			got := queueFromContext(ctx, logger)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("queueFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_toInternalThreadFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   []string
	}{
		{name: "nil stays nil", fields: nil, want: nil},
		{name: "no channel is unchanged", fields: []string{"id", "members"}, want: []string{"id", "members"}},
		{name: "channel becomes source", fields: []string{"id", "channel"}, want: []string{"id", "source"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInternalThreadFields(tt.fields); !slices.Equal(got, tt.want) {
				t.Errorf("toInternalThreadFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_toInternalThreadSort(t *testing.T) {
	tests := map[string]string{
		"":              "",
		"+channel_name": "+source_name",
		"-channel_name": "-source_name",
		"-queue_name":   "-queue_name",
		"+updated_at":   "+updated_at",
	}
	for in, want := range tests {
		if got := toInternalThreadSort(in); got != want {
			t.Errorf("toInternalThreadSort(%q) = %q, want %q", in, got, want)
		}
	}
}

func Test_newSearchLeftRequest(t *testing.T) {
	identity := readCallerIdentity{contactID: "agent-1"}

	tests := []struct {
		name string
		req  *gtwthread.SearchLeftRequest
		want *threadv1.SearchLeftRequest
	}{
		{
			name: "channel field and channel_name sort map to source",
			req: &gtwthread.SearchLeftRequest{
				Fields: []string{"id", "channel", "queue"},
				Sort:   "+channel_name",
				Types:  []gtwthread.ThreadKind{gtwthread.ThreadKind_DIRECT},
				Size:   20,
				Page:   2,
			},
			want: &threadv1.SearchLeftRequest{
				Fields:   []string{"id", "source", "queue"},
				MemberId: "agent-1",
				DomainId: 1,
				Kinds:    []threadv1.ThreadKind{threadv1.ThreadKind_DIRECT},
				Size:     20,
				Sort:     "+source_name",
				Page:     2,
			},
		},
		{
			name: "descending channel_name sort maps to source_name",
			req:  &gtwthread.SearchLeftRequest{Sort: "-channel_name"},
			want: &threadv1.SearchLeftRequest{MemberId: "agent-1", DomainId: 1, Sort: "-source_name"},
		},
		{
			name: "queue_name sort and queue field pass through",
			req: &gtwthread.SearchLeftRequest{
				Fields: []string{"queue"},
				Sort:   "-queue_name",
			},
			want: &threadv1.SearchLeftRequest{
				Fields:   []string{"queue"},
				MemberId: "agent-1",
				DomainId: 1,
				Sort:     "-queue_name",
			},
		},
		{
			name: "nil fields and empty sort stay unset",
			req:  &gtwthread.SearchLeftRequest{},
			want: &threadv1.SearchLeftRequest{MemberId: "agent-1", DomainId: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSearchLeftRequest(tt.req, identity)
			if err != nil {
				t.Fatalf("newSearchLeftRequest() error = %v", err)
			}
			if !proto.Equal(got, tt.want) {
				t.Errorf("newSearchLeftRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newSearchLeftRequest_doesNotMutateRequest(t *testing.T) {
	req := &gtwthread.SearchLeftRequest{Fields: []string{"id", "channel"}}

	_, _ = newSearchLeftRequest(req, readCallerIdentity{contactID: "agent-1"})

	if !slices.Equal(req.GetFields(), []string{"id", "channel"}) {
		t.Errorf("newSearchLeftRequest() mutated request fields: %v", req.GetFields())
	}
}

type domainCallerIdentity struct {
	readCallerIdentity

	domainID int64
}

func (i domainCallerIdentity) GetDomainID() int64 { return i.domainID }

func Test_newSearchLeftRequest_domainIDRange(t *testing.T) {
	tests := []struct {
		name     string
		domainID int64
		wantErr  bool
	}{
		{name: "max int32 fits", domainID: math.MaxInt32},
		{name: "min int32 fits", domainID: math.MinInt32},
		{name: "above int32 overflows", domainID: math.MaxInt32 + 1, wantErr: true},
		{name: "below int32 overflows", domainID: math.MinInt32 - 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := domainCallerIdentity{readCallerIdentity: readCallerIdentity{contactID: "agent-1"}, domainID: tt.domainID}

			got, err := newSearchLeftRequest(&gtwthread.SearchLeftRequest{}, identity)
			if (err != nil) != tt.wantErr {
				t.Fatalf("newSearchLeftRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					t.Errorf("newSearchLeftRequest() = %v, want nil on error", got)
				}

				return
			}

			if int64(got.GetDomainId()) != tt.domainID {
				t.Errorf("newSearchLeftRequest().DomainId = %d, want %d", got.GetDomainId(), tt.domainID)
			}
		})
	}
}

type fakeChannelResolver struct {
	got *threadv1.Thread
	ch  *gtwthread.ThreadChannel
}

func (f *fakeChannelResolver) Resolve(_ context.Context, _ int64, thr *threadv1.Thread, _ map[string]*contactv1.Contact) *gtwthread.ThreadChannel {
	f.got = thr

	return f.ch
}

func Test_thread_withChannel_leftItem(t *testing.T) {
	gate := &gtwthread.ThreadChannel{
		Id:   "gate-1",
		Name: "Support",
		Type: gtwthread.ThreadChannelType_THREAD_CHANNEL_TYPE_TELEGRAM_BOT,
	}

	t.Run("queue and channel resolved from source", func(t *testing.T) {
		res := &fakeChannelResolver{ch: gate}
		th := &thread{channels: res}
		item := &threadv1.Thread{
			Id:     "dlg-1",
			Source: &threadv1.ThreadSource{Id: "gate-1", Name: "Support", Type: "telegram_bot"},
			Queue:  &threadv1.ThreadQueue{Id: 7, Name: "Sales"},
		}

		got := th.withChannel(context.Background(), 1, item, nil)

		if !proto.Equal(got.GetQueue(), &gtwthread.ThreadQueue{Id: 7, Name: "Sales"}) {
			t.Errorf("withChannel().Queue = %v, want {7 Sales}", got.GetQueue())
		}
		if !proto.Equal(got.GetChannel(), gate) {
			t.Errorf("withChannel().Channel = %v, want %v", got.GetChannel(), gate)
		}
		if res.got != item {
			t.Errorf("resolver got thread %p, want %p", res.got, item)
		}
	})

	t.Run("nil queue and nil resolver result", func(t *testing.T) {
		res := &fakeChannelResolver{}
		th := &thread{channels: res}

		got := th.withChannel(context.Background(), 1, &threadv1.Thread{Id: "dlg-2"}, nil)

		if got.GetQueue() != nil {
			t.Errorf("withChannel().Queue = %v, want nil", got.GetQueue())
		}
		if got.GetChannel() != nil {
			t.Errorf("withChannel().Channel = %v, want nil", got.GetChannel())
		}
	})

	t.Run("nil resolver keeps queue", func(t *testing.T) {
		th := &thread{}
		item := &threadv1.Thread{Id: "dlg-3", Queue: &threadv1.ThreadQueue{Id: 9, Name: "Queue"}}

		got := th.withChannel(context.Background(), 1, item, nil)

		if got.GetChannel() != nil {
			t.Errorf("withChannel().Channel = %v, want nil", got.GetChannel())
		}
		if !proto.Equal(got.GetQueue(), &gtwthread.ThreadQueue{Id: 9, Name: "Queue"}) {
			t.Errorf("withChannel().Queue = %v, want {9 Queue}", got.GetQueue())
		}
	})
}
