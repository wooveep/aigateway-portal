package portal

import "testing"

func TestAppendKeyAuthCredentialsIncludesBearerAlias(t *testing.T) {
	t.Parallel()

	credentials := appendKeyAuthCredentials(nil, "KEY123", "hgpk_live_test")
	if len(credentials) != 2 {
		t.Fatalf("appendKeyAuthCredentials() len = %d, want 2", len(credentials))
	}
	if credentials[0].Credential != "hgpk_live_test" {
		t.Fatalf("unexpected raw credential: %s", credentials[0].Credential)
	}
	if credentials[1].Credential != "Bearer hgpk_live_test" {
		t.Fatalf("unexpected bearer credential: %s", credentials[1].Credential)
	}
	if credentials[0].KeyID != "KEY123" || credentials[1].KeyID != "KEY123" {
		t.Fatalf("unexpected key ids: %+v", credentials)
	}
}

func TestAppendKeyAuthCredentialsDeduplicatesExistingEntries(t *testing.T) {
	t.Parallel()

	existing := []keyAuthCredential{
		{KeyID: "KEY123", Credential: "hgpk_live_test"},
		{KeyID: "KEY123", Credential: "Bearer hgpk_live_test"},
	}
	credentials := appendKeyAuthCredentials(existing, "KEY123", "hgpk_live_test")
	if len(credentials) != 2 {
		t.Fatalf("appendKeyAuthCredentials() len = %d, want 2", len(credentials))
	}
}
