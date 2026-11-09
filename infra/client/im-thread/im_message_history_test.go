package imthread

import (
	"testing"

	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func TestToSearchMessageHistoryRequest_SystemCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		systemCall bool
		wantSet    bool
	}{
		{name: "member read leaves system_call unset", systemCall: false, wantSet: false},
		{name: "system read sets system_call", systemCall: true, wantSet: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := toSearchMessageHistoryRequest(&dto.SearchMessageHistoryRequest{
				ThreadIDs:  []string{"thread"},
				DomainID:   1,
				CallerID:   "caller",
				SystemCall: tt.systemCall,
			})

			if got := req.SystemCall != nil; got != tt.wantSet {
				t.Fatalf("system_call set = %v, want %v", got, tt.wantSet)
			}

			if req.GetSystemCall() != tt.systemCall {
				t.Fatalf("system_call = %v, want %v", req.GetSystemCall(), tt.systemCall)
			}

			if req.GetDomainId() != 1 || req.GetCallerId() != "caller" || req.GetThreadId() != "thread" {
				t.Fatalf("unexpected request: %+v", req)
			}
		})
	}
}
