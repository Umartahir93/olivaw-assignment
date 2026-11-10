//go:build unit

package domain

import (
	"testing"
	"time"
)

func TestConnectivityAt(t *testing.T) {
	now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		lastSeen *time.Time
		want     Connectivity
	}{
		{
			name:     "never seen",
			lastSeen: nil,
			want:     ConnectivityUnknown,
		},
		{
			name:     "just now",
			lastSeen: ptr(now),
			want:     ConnectivityOnline,
		},
		{
			name:     "exactly 5 minutes ago",
			lastSeen: ptr(now.Add(-OfflineAfter)),
			want:     ConnectivityOnline,
		},
		{
			name:     "just over 5 minutes ago",
			lastSeen: ptr(now.Add(-OfflineAfter - time.Second)),
			want:     ConnectivityOffline,
		},
		{
			name:     "an hour ago",
			lastSeen: ptr(now.Add(-time.Hour)),
			want:     ConnectivityOffline,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConnectivityAt(tt.lastSeen, now); got != tt.want {
				t.Errorf("ConnectivityAt = %v, want %v", got, tt.want)
			}
			if got := (Gateway{LastSeenAt: tt.lastSeen}).Connectivity(now); got != tt.want {
				t.Errorf("Gateway.Connectivity = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
