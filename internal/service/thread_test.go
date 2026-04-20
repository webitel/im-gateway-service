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
			if reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToThread() = got %v, want %v", got, tt.want)
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
