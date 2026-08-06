package imthread

import (
	"testing"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
)

func TestMapRecipientStatuses(t *testing.T) {
	in := []*threadv1.MessageRecipientStatus{
		nil, // nil entries are skipped
		{
			MemberId:    "019c711c-58cb-7f58-a6c7-80fb6f49d046",
			Status:      threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_READ,
			DeliveredAt: 1_770_000_000_000,
			ReadAt:      1_770_000_100_000,
			Via:         "ws",
		},
		{
			MemberId: "c7be03da-bdb3-4332-b595-9636ddf5f5b9",
			Status:   threadv1.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_FAILED,
			FailedAt: 1_770_000_200_000,
			Via:      "provider",
			Error:    `{"code":"131047"}`,
		},
	}

	out := MapRecipientStatuses(in)

	if len(out) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(out))
	}

	read := out[0]

	if read.Status != api.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_READ {
		t.Errorf("expected READ, got %v", read.Status)
	}

	if read.MemberId != in[1].MemberId || read.DeliveredAt != in[1].DeliveredAt || read.ReadAt != in[1].ReadAt || read.Via != "ws" {
		t.Errorf("read status fields mismatch: %+v", read)
	}

	failed := out[1]

	if failed.Status != api.MessageDeliveryStatus_MESSAGE_DELIVERY_STATUS_FAILED {
		t.Errorf("expected FAILED, got %v", failed.Status)
	}

	if failed.FailedAt != in[2].FailedAt || failed.Error != `{"code":"131047"}` {
		t.Errorf("failed status fields mismatch: %+v", failed)
	}
}

func TestMapRecipientStatuses_Empty(t *testing.T) {
	if got := MapRecipientStatuses(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}

	if got := MapRecipientStatuses([]*threadv1.MessageRecipientStatus{}); got != nil {
		t.Errorf("expected nil for zero-length input, got %v", got)
	}
}
