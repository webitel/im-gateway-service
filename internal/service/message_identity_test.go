package service

import (
	"testing"

	"google.golang.org/protobuf/proto"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth/standard"
)

func Test_senderIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity *standard.Identity
		want     *threadv1.Identity
	}{
		{
			name:     "full via sets via and via_ref",
			identity: &standard.Identity{Via: "gate-1", ViaName: "Підтримка", ViaType: "facebook", Name: "N", ChatName: "C"},
			want: &threadv1.Identity{
				Name:     "N",
				ChatName: "C",
				Via:      ptr("gate-1"), //nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref
				ViaRef:   &threadv1.Via{Id: "gate-1", Name: "Підтримка", Type: "facebook"},
			},
		},
		{
			name:     "id only via (old provider)",
			identity: &standard.Identity{Via: "gate-1", Name: "N"},
			want: &threadv1.Identity{
				Name:   "N",
				Via:    ptr("gate-1"), //nolint:staticcheck // legacy field kept until every im-thread-service reads via_ref
				ViaRef: &threadv1.Via{Id: "gate-1"},
			},
		},
		{
			name:     "no via (end user call)",
			identity: &standard.Identity{Name: "N", ChatName: "C"},
			want: &threadv1.Identity{
				Name:     "N",
				ChatName: "C",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := senderIdentity(tt.identity)
			if !proto.Equal(got, tt.want) {
				t.Errorf("senderIdentity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_viaRef(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want *threadv1.Via
	}{
		{name: "empty id", id: "", want: nil},
		{name: "id only", id: "gate-1", want: &threadv1.Via{Id: "gate-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := viaRef(tt.id, "", "")
			if !proto.Equal(got, tt.want) {
				t.Errorf("viaRef() = %v, want %v", got, tt.want)
			}
		})
	}
}
