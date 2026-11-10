//go:build unit

package domain_test

import (
	"testing"
	"time"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

func TestConnectivityAt(t *testing.T) {
	now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		lastSeen *time.Time
		want     domain.Connectivity
	}{
		{
			name:     "never seen",
			lastSeen: nil,
			want:     domain.ConnectivityUnknown,
		},
		{
			name:     "just now",
			lastSeen: ptr(now),
			want:     domain.ConnectivityOnline,
		},
		{
			name:     "exactly 5 minutes ago",
			lastSeen: ptr(now.Add(-domain.OfflineAfter)),
			want:     domain.ConnectivityOnline,
		},
		{
			name:     "just over 5 minutes ago",
			lastSeen: ptr(now.Add(-domain.OfflineAfter - time.Second)),
			want:     domain.ConnectivityOffline,
		},
		{
			name:     "an hour ago",
			lastSeen: ptr(now.Add(-time.Hour)),
			want:     domain.ConnectivityOffline,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ConnectivityAt(tt.lastSeen, now); got != tt.want {
				t.Errorf("ConnectivityAt = %v, want %v", got, tt.want)
			}
			if got := (domain.Gateway{LastSeenAt: tt.lastSeen}).Connectivity(now); got != tt.want {
				t.Errorf("Gateway.Connectivity = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
