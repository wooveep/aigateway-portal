package portal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyPasswordSupportsBcryptAndLegacySHA256(t *testing.T) {
	bcryptHash, err := hashPassword("8DC563ED")
	require.NoError(t, err)

	matched, needsUpgrade := verifyPassword(bcryptHash, "8DC563ED")
	require.True(t, matched)
	require.False(t, needsUpgrade)

	matched, needsUpgrade = verifyPassword(legacyPasswordHash("8DC563ED"), "8DC563ED")
	require.True(t, matched)
	require.True(t, needsUpgrade)

	matched, needsUpgrade = verifyPassword(legacyPasswordHash("8DC563ED"), "wrong-password")
	require.False(t, matched)
	require.False(t, needsUpgrade)
}
