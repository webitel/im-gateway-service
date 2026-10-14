package service

import (
	"slices"
	"testing"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// The failed recipient is looked up and resolved like any other contact a history message references.
func TestHistoryFailures_ResolveMember(t *testing.T) {
	msg := &dto.HistoryMessage{ID: "msg", Failures: []*dto.MessageFailure{{MemberID: "c1", ErrorCode: "403"}}}

	if ids := historyExtraContactIDs([]*dto.HistoryMessage{msg}); !slices.Contains(ids, "c1") {
		t.Fatalf("extra contact ids = %v, want c1", ids)
	}

	imap := map[string]*dto.MessageSender{"c1": {ContactID: "c1", Name: "Ivan", MemberID: "m1"}}
	enrichMessages([]*dto.HistoryMessage{msg}, imap)

	if got := msg.Failures[0].Member; got == nil || got.Name != "Ivan" || got.MemberID != "m1" {
		t.Fatalf("failure member = %+v, want resolved Ivan", got)
	}
}

// last_msg failures resolve to thread members; a recipient who has left keeps just the contact.
func TestConvertToThread_LastMessageFailures(t *testing.T) {
	thr := &threadv1.Thread{
		Id:      "t1",
		Members: []*threadv1.ThreadMember{{Id: "m1", ContactId: "c1"}},
		LastMsg: &threadv1.HistoryMessage{Id: "msg", Failures: []*threadv1.DeliveryFailure{
			{MemberId: "c1", Error: &threadv1.DeliveryError{Code: "403", Message: "blocked"}},
			{MemberId: "c2", Error: &threadv1.DeliveryError{Code: "410"}},
		}},
	}
	contacts := map[string]*contactv1.Contact{
		"c1": {Id: "c1", Name: "Ivan"},
		"c2": {Id: "c2", Name: "Gone"},
	}

	if ids := new(thread).collectUniqueContactsFromThread([]*threadv1.Thread{thr}); !slices.Contains(ids, "c2") {
		t.Fatalf("contacts to fetch = %v, want the departed c2", ids)
	}

	got := convertToThread(thr, contacts).GetLastMsg().GetFailures()
	if len(got) != 2 {
		t.Fatalf("failures = %d, want 2", len(got))
	}

	if got[0].GetMember().GetId() != "m1" || got[0].GetMember().GetContact().GetName() != "Ivan" ||
		got[0].GetError().GetCode() != "403" || got[0].GetError().GetMessage() != "blocked" {
		t.Errorf("member failure = %+v", got[0])
	}

	if got[1].GetMember().GetId() != "" || got[1].GetMember().GetContact().GetName() != "Gone" || got[1].GetError().GetCode() != "410" {
		t.Errorf("departed failure = %+v", got[1])
	}
}
