package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuotaCreditsConversion(t *testing.T) {
	require.Equal(t, "0", QuotaToCreditsString(0))
	require.Equal(t, "1", QuotaToCreditsString(3600))
	require.Equal(t, "20", QuotaToCreditsString(72000))
	require.Equal(t, "0.5", QuotaToCreditsString(1800))
	require.Equal(t, "0.000278", QuotaToCreditsString(1))

	quota, ok := CreditsToQuota(275000)
	require.True(t, ok)
	require.Equal(t, 990000000, quota)
}

func TestCreditsV1DefaultsOff(t *testing.T) {
	t.Setenv(CreditsFeatureFlagEnv, "")
	require.False(t, CreditsV1Enabled())
	t.Setenv(CreditsFeatureFlagEnv, "true")
	require.True(t, CreditsV1Enabled())
}
