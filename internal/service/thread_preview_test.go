package service

import (
	"context"
	"reflect"
	"testing"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwpreview "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/infra/auth/standard"
)

func Test_convertToThreadPreview(t *testing.T) {
	target := &contactv1.Contact{Id: "c-target", Subject: "sub-target", IssId: "iss"}
	initiator := &contactv1.Contact{Id: "c-init", Subject: "sub-init", IssId: "iss"}

	tests := []struct {
		name      string
		preview   *threadv1.ThreadPreview
		target    *contactv1.Contact
		initiator *contactv1.Contact
		want      *gtwpreview.ThreadPreview
	}{
		{
			name:    "nil preview",
			preview: nil,
			target:  target,
			want:    nil,
		},
		{
			name: "nil initiator leaves initiator unset",
			preview: &threadv1.ThreadPreview{
				Id:                 "p1",
				ThreadId:           "t1",
				ContactId:          "c-target",
				InitiatorContactId: "c-init",
				DomainId:           7,
				CreatedAt:          100,
				ExpiresAt:          200,
			},
			target:    target,
			initiator: nil,
			want: &gtwpreview.ThreadPreview{
				Id:        "p1",
				ThreadId:  "t1",
				Contact:   convertToContact(target),
				CreatedAt: 100,
				ExpiresAt: 200,
			},
		},
		{
			name: "full input maps all fields without domain id",
			preview: &threadv1.ThreadPreview{
				Id:                 "p2",
				ThreadId:           "t2",
				ContactId:          "c-target",
				InitiatorContactId: "c-init",
				DomainId:           7,
				CreatedAt:          300,
				ExpiresAt:          400,
			},
			target:    target,
			initiator: initiator,
			want: &gtwpreview.ThreadPreview{
				Id:        "p2",
				ThreadId:  "t2",
				Contact:   convertToContact(target),
				Initiator: convertToContact(initiator),
				CreatedAt: 300,
				ExpiresAt: 400,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToThreadPreview(tt.preview, tt.target, tt.initiator)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToThreadPreview() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_ThreadPreviewService_RejectsNonSystemCaller(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, auth.Identifier(&standard.Identity{}))

	// Nil clients: any downstream call would panic, so an error here proves the early rejection.
	svc := NewThreadPreviewService(nil, nil, nil)

	if _, err := svc.GivePreview(ctx, &gtwpreview.GivePreviewRequest{}); err == nil {
		t.Error("GivePreview() for end user: expected error, got nil")
	}
	if _, err := svc.RemoveFromPreview(ctx, &gtwpreview.RemoveFromPreviewRequest{}); err == nil {
		t.Error("RemoveFromPreview() for end user: expected error, got nil")
	}
	if _, err := svc.UpgradePreviewToFullMember(ctx, &gtwpreview.UpgradePreviewToFullMemberRequest{}); err == nil {
		t.Error("UpgradePreviewToFullMember() for end user: expected error, got nil")
	}
}
