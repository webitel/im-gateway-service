package service

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"testing"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/model"
)

var (
	contactData = map[string]*contactv1.Contact{
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
)

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
							Iss:      "1",
							Sub:      "2",
							Username: "two",
						},
					},
				},
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
		want        []*gtwthread.Thread
		want2       bool
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := NewThread(tt.logger, tt.threadClient, tt.contactClient)
			got, got2, gotErr := th.Search(context.Background(), tt.searchQuery)
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
			if true {
				t.Errorf("Search() = %v, want %v", got2, tt.want2)
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
