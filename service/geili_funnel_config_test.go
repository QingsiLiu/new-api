package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeiliFunnelConfigFailsClosed(t *testing.T) {
	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", "short")
	_, err := LoadGeiliFunnelConfig()
	require.ErrorIs(t, err, ErrGeiliFunnelSecretInvalid)

	t.Setenv("GEILI_FUNNEL_ENABLED", "false")
	config, err := LoadGeiliFunnelConfig()
	require.NoError(t, err)
	require.False(t, config.Enabled)

	t.Setenv("GEILI_FUNNEL_ENABLED", "true")
	t.Setenv("GEILI_FUNNEL_SECRET", strings.Repeat("s", 32))
	t.Setenv("GEILI_FUNNEL_UNAUTHORIZED_RPM", "0")
	t.Setenv("GEILI_FUNNEL_WRITE_RPM", "-1")
	t.Setenv("GEILI_FUNNEL_READ_RPM", "invalid")
	config, err = LoadGeiliFunnelConfig()
	require.NoError(t, err)
	require.Equal(t, 60, config.UnauthorizedRPM)
	require.Equal(t, 1200, config.WriteRPM)
	require.Equal(t, 60, config.ReadRPM)
}
