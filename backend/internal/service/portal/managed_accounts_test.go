package portal

import "testing"

func TestNewManagedAccountTempPassword(t *testing.T) {
	t.Parallel()

	value := newManagedAccountTempPassword()
	if len(value) != 8 {
		t.Fatalf("newManagedAccountTempPassword() length = %d, want 8", len(value))
	}
}
