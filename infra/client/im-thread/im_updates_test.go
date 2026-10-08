package imthread

import (
	"testing"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
)

func TestToUpdatesResponseDTO(t *testing.T) {
	out := ToUpdatesResponseDTO(&threadv1.GetUpdatesResponse{
		Cursor: "900",
		Threads: []*threadv1.ThreadUpdates{{
			ThreadId: "t1", UnreadCount: 2,
			Dialog:            &threadv1.Thread{Id: "t1"},
			TopMessage:        &threadv1.UpdatedMessage{Id: "top", SenderId: "c1"},
			Messages:          []*threadv1.UpdatedMessage{{Id: "m1", Seq: 2, SenderId: "c1", Body: "current", EditedAt: 1700}},
			DeletedMessageIds: []string{"m0"},
			MemberChanges: []*threadv1.ThreadMemberChange{{
				ContactId: "c2", ById: "c1", Action: threadv1.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_LEFT,
			}},
			ReadStates: []*threadv1.MemberReadState{{MemberId: "c2", DeliveredUpToSeq: 5, ReadUpToSeq: 4}},
			Members:    []*threadv1.ThreadMember{{ContactId: "c1"}},
		}},
	})

	if out.Cursor != "900" || out.Resync || len(out.Threads) != 1 {
		t.Fatalf("header = %+v", out)
	}

	th := out.Threads[0]
	if th.ThreadID != "t1" || th.UnreadCount != 2 || th.RawDialog.GetId() != "t1" || len(th.From) != 1 {
		t.Errorf("thread = %+v", th)
	}

	if m := th.Messages[0]; m.ID != "m1" || m.Body != "current" || m.UpdatedAt != 1700 || m.SenderID != "c1" {
		t.Errorf("message = %+v", m)
	}

	if th.TopMessage == nil || th.TopMessage.ID != "top" {
		t.Errorf("top message = %+v", th.TopMessage)
	}

	if mc := th.MemberChanges[0]; mc.ContactID != "c2" || mc.ByID != "c1" || mc.Action != int32(api.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_LEFT) {
		t.Errorf("member change = %+v", mc)
	}

	if rs := th.ReadStates[0]; rs.DeliveredUpToSeq != 5 || rs.ReadUpToSeq != 4 {
		t.Errorf("read state = %+v", rs)
	}
}

// A failed message keeps its failures through the history mapping updates reuse.
func TestToUpdatesResponseDTO_MessageFailures(t *testing.T) {
	out := ToUpdatesResponseDTO(&threadv1.GetUpdatesResponse{Threads: []*threadv1.ThreadUpdates{{
		ThreadId: "t1",
		Messages: []*threadv1.UpdatedMessage{{Id: "m1", Failures: []*threadv1.DeliveryFailure{
			{MemberId: "c2", Error: &threadv1.DeliveryError{Code: "403", Message: "blocked"}},
		}}},
	}}})

	got := out.Threads[0].Messages[0].Failures
	if len(got) != 1 || got[0].MemberID != "c2" || got[0].ErrorCode != "403" || got[0].ErrorMessage != "blocked" {
		t.Fatalf("failures = %+v", got)
	}
}
