package operation_setting

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAsyncFailoverDefaultsAreSafe(t *testing.T) {
	defaults := AsyncFailoverDefaultOptions()
	require.Equal(t, "false", defaults[AsyncFailoverEnabledOption])
	require.Equal(t, "3", defaults[AsyncFailoverMaxAttemptsOption])
	require.Equal(t, "3", defaults[AsyncPollTransientRetriesOption])
	require.Equal(t, "false", defaults[AsyncCircuitEnabledOption])
	require.Equal(t, "180", defaults[AsyncTaskAttemptRetentionDaysOption])
}

func TestValidateAsyncFailoverOptions(t *testing.T) {
	require.NoError(t, ValidateAsyncFailoverOption(AsyncFailoverMaxAttemptsOption, "3"))
	require.Error(t, ValidateAsyncFailoverOption(AsyncFailoverMaxAttemptsOption, "0"))
	require.Error(t, ValidateAsyncFailoverOption(AsyncFailoverMaxAttemptsOption, "4"))
	require.Error(t, ValidateAsyncFailoverOption(AsyncCircuitEnabledOption, "not-a-bool"))
	require.NoError(t, ValidateAsyncFailoverOption(AsyncCircuitSuccessRateThresholdOption, "40"))
	require.Error(t, ValidateAsyncFailoverOption(AsyncCircuitSuccessRateThresholdOption, "101"))
}

func TestUpdateAsyncFailoverOptionPublishesSnapshot(t *testing.T) {
	original := GetAsyncFailoverSetting()
	t.Cleanup(func() {
		_ = UpdateAsyncFailoverOption(AsyncFailoverEnabledOption, strconv.FormatBool(original.Enabled))
		_ = UpdateAsyncFailoverOption(AsyncFailoverMaxAttemptsOption, strconv.Itoa(original.MaxAttempts))
		_ = UpdateAsyncFailoverOption(AsyncCircuitEnabledOption, strconv.FormatBool(original.CircuitEnabled))
	})

	require.NoError(t, UpdateAsyncFailoverOption(AsyncFailoverEnabledOption, "true"))
	require.NoError(t, UpdateAsyncFailoverOption(AsyncFailoverMaxAttemptsOption, "2"))
	require.NoError(t, UpdateAsyncFailoverOption(AsyncCircuitEnabledOption, "true"))

	snapshot := GetAsyncFailoverSetting()
	require.True(t, snapshot.Enabled)
	require.Equal(t, 2, snapshot.MaxAttempts)
	require.True(t, snapshot.CircuitEnabled)
}
