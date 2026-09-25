package mapper_test

import (
	"testing"

	pb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// Contacts in updates use history's ThreadMember shape; messages keep only UI fields.
func TestMapToGetUpdatesProto(t *testing.T) {
	sender := &dto.MessageSender{ContactID: "c1", Sub: "3", Iss: "webitel", Type: "webitel", Name: "Admin", Username: "adm", MemberID: "m1", Role: 3}

	got := mapper.MapToGetUpdatesProto(&dto.GetUpdatesResponse{
		Cursor: "900",
		Threads: []*dto.ThreadUpdates{{
			ThreadID:          "t1",
			Cursor:            "5",
			Dialog:            &pb.Thread{Id: "t1", Subject: "new"},
			TopMessage:        &dto.HistoryMessage{ID: "top", Sender: sender},
			Messages:          []*dto.HistoryMessage{{ID: "msg", Seq: 2, Body: "hi", Sender: sender, UpdatedAt: 1700}},
			DeletedMessageIDs: []string{"gone"},
			MemberChanges:     []*dto.ThreadMemberChange{{Member: sender, By: sender, Action: int32(pb.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_JOINED)}},
			ReadStates:        []*dto.MemberReadState{{MemberID: "c1", ReadUpToSeq: 1, Member: sender}},
		}},
	})

	if got.GetCursor() != "900" || got.GetResync() || len(got.GetThreads()) != 1 {
		t.Fatalf("header = %+v", got)
	}

	th := got.GetThreads()[0]
	if th.GetCursor() != "5" || th.GetDialog().GetSubject() != "new" || th.GetTopMessage().GetId() != "top" || len(th.GetDeletedMessageIds()) != 1 {
		t.Fatalf("thread = %+v", th)
	}

	msg := th.GetMessages()[0]
	if msg.GetId() != "msg" || msg.GetSeq() != 2 || msg.GetBody() != "hi" || msg.GetEditedAt() != 1700 {
		t.Errorf("message = %+v", msg)
	}

	for name, m := range map[string]*pb.ThreadMember{
		"sender":      msg.GetSender(),
		"member":      th.GetMemberChanges()[0].GetMember(),
		"by":          th.GetMemberChanges()[0].GetBy(),
		"read member": th.GetReadStates()[0].GetMember(),
	} {
		c := m.GetContact()
		if m.GetId() != "m1" || m.GetRole() != pb.ThreadRole_ROLE_OWNER || c.GetSub() != "3" || c.GetName() != "Admin" || c.GetUsername() != "adm" {
			t.Errorf("%s = %+v, want history-shaped ThreadMember", name, m)
		}
	}

	if th.GetMemberChanges()[0].GetAction() != pb.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_JOINED {
		t.Errorf("action = %v", th.GetMemberChanges()[0].GetAction())
	}
}
