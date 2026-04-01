package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNowTextFormatsUTCInstant(t *testing.T) {
	value := time.Date(2026, time.April, 1, 8, 9, 10, 123000000, time.FixedZone("UTC+8", 8*60*60))
	require.Equal(t, "2026-04-01T00:09:10.123Z", NowText(value))
}

func TestParseDateTimeTreatsNaiveLocalInputAsUTC(t *testing.T) {
	parsed, err := ParseDateTime("2026-04-01T10:30")
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, time.Date(2026, time.April, 1, 10, 30, 0, 0, time.UTC), parsed.UTC())
}
