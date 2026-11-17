package service

import (
	"math"
	"testing"
)

func Test_domainIDInt32(t *testing.T) {
	tests := []struct {
		name     string
		domainID int64
		want     int32
		wantErr  bool
	}{
		{name: "regular id", domainID: 1, want: 1},
		{name: "max int32 fits", domainID: math.MaxInt32, want: math.MaxInt32},
		{name: "min int32 fits", domainID: math.MinInt32, want: math.MinInt32},
		{name: "above int32 overflows", domainID: math.MaxInt32 + 1, wantErr: true},
		{name: "below int32 overflows", domainID: math.MinInt32 - 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domainIDInt32(domainCallerIdentity{domainID: tt.domainID})
			if (err != nil) != tt.wantErr {
				t.Fatalf("domainIDInt32() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("domainIDInt32() = %d, want %d", got, tt.want)
			}
		})
	}
}

func Test_sizeInt32(t *testing.T) {
	if got := sizeInt32(10); got != 10 {
		t.Errorf("sizeInt32(10) = %d, want 10", got)
	}

	if got := sizeInt32(math.MaxInt32 + 1); got != math.MaxInt32 {
		t.Errorf("sizeInt32(MaxInt32+1) = %d, want %d", got, math.MaxInt32)
	}
}
