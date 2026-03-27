package portal

import (
	"testing"
	"time"

	"higress-portal-backend/internal/consts"
	"higress-portal-backend/internal/model"
)

func TestIsUsableAPIKey(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	deleted := now.Add(-time.Minute)

	tests := []struct {
		name string
		row  model.APIKeyRow
		want bool
	}{
		{
			name: "active and unexpired",
			row: model.APIKeyRow{
				Status:    consts.APIKeyStatusActive,
				ExpiresAt: &future,
			},
			want: true,
		},
		{
			name: "disabled status",
			row: model.APIKeyRow{
				Status:    consts.APIKeyStatusDisabled,
				ExpiresAt: &future,
			},
			want: false,
		},
		{
			name: "expired key",
			row: model.APIKeyRow{
				Status:    consts.APIKeyStatusActive,
				ExpiresAt: &past,
			},
			want: false,
		},
		{
			name: "soft deleted key",
			row: model.APIKeyRow{
				Status:    consts.APIKeyStatusActive,
				ExpiresAt: &future,
				DeletedAt: &deleted,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isUsableAPIKey(tt.row, now); got != tt.want {
				t.Fatalf("isUsableAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
