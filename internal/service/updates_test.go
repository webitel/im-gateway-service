package service

import (
	"context"
	"slices"
	"testing"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type fakeContacts struct {
	calls int
	asked []string
	list  []*contactv1.Contact
}

func (f *fakeContacts) SearchContact(_ context.Context, req *contactv1.SearchContactRequest) (*contactv1.ContactList, error) {
	f.calls++
	f.asked = req.GetIds()

	return &contactv1.ContactList{Contacts: f.list}, nil
}

// One contact search resolves every thread; roles come from each thread's own members.
func TestUpdatesEnrich(t *testing.T) {
	contacts := &fakeContacts{list: []*contactv1.Contact{
		{Id: "alice", Name: "Alice", Subject: "1"},
		{Id: "bob", Username: "bob"},
	}}
	s := &updates{contacts: contacts}

	owned := &dto.ThreadUpdates{
		From:          []*threadv1.ThreadMember{{Id: "m-alice", ContactId: "alice", Role: threadv1.ThreadRole_ROLE_OWNER}},
		Messages:      []*dto.HistoryMessage{{ID: "1", SenderID: "alice", ReplyTo: &dto.HistoryReplyTo{SenderID: "bob"}}},
		MemberChanges: []*dto.ThreadMemberChange{{ContactID: "bob", ByID: "alice"}},
		ReadStates:    []*dto.MemberReadState{{MemberID: "bob"}},
	}
	fresh := &dto.ThreadUpdates{
		From:       []*threadv1.ThreadMember{{Id: "m-alice-2", ContactId: "alice", Role: threadv1.ThreadRole_ROLE_MEMBER}},
		TopMessage: &dto.HistoryMessage{ID: "2", SenderID: "alice"},
		RawDialog: &threadv1.Thread{Id: "t2", Subject: "new", Members: []*threadv1.ThreadMember{
			{Id: "m-alice-2", ContactId: "alice"},
		}},
	}

	if err := s.enrich(context.Background(), 1, []*dto.ThreadUpdates{owned, fresh}); err != nil {
		t.Fatal(err)
	}

	if contacts.calls != 1 || !slices.Equal(contacts.asked, []string{"alice", "bob"}) {
		t.Fatalf("contact search = %d calls for %v, want one for alice and bob", contacts.calls, contacts.asked)
	}

	sender := owned.Messages[0].Sender
	if sender.Name != "Alice" || sender.MemberID != "m-alice" || sender.Role != int(threadv1.ThreadRole_ROLE_OWNER) {
		t.Errorf("sender = %+v", sender)
	}

	if owned.Messages[0].ReplyTo.Sender.Name != "bob" || owned.MemberChanges[0].Member.ContactID != "bob" ||
		owned.MemberChanges[0].By.ContactID != "alice" || owned.ReadStates[0].Member.ContactID != "bob" {
		t.Errorf("references not resolved: %+v %+v", owned.MemberChanges[0], owned.ReadStates[0])
	}

	if fresh.TopMessage.Sender.MemberID != "m-alice-2" || fresh.TopMessage.Sender.Role != int(threadv1.ThreadRole_ROLE_MEMBER) {
		t.Errorf("top message sender = %+v", fresh.TopMessage.Sender)
	}

	if fresh.Dialog.GetSubject() != "new" || fresh.Dialog.GetMembers()[0].GetContact().GetName() != "Alice" {
		t.Errorf("dialog = %+v", fresh.Dialog)
	}
}

func TestUpdatesEnrich_NothingToResolve(t *testing.T) {
	contacts := &fakeContacts{}

	if err := (&updates{contacts: contacts}).enrich(context.Background(), 1, []*dto.ThreadUpdates{{Left: true}}); err != nil {
		t.Fatal(err)
	}

	if contacts.calls != 0 {
		t.Errorf("contact search called %d times for a left thread", contacts.calls)
	}
}
