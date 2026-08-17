package operation_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	AsyncFailoverEnabledOption              = "AsyncFailoverEnabled"
	AsyncFailoverMaxAttemptsOption          = "AsyncFailoverMaxAttempts"
	AsyncPollTransientRetriesOption         = "AsyncPollTransientRetries"
	AsyncCircuitEnabledOption               = "AsyncCircuitEnabled"
	AsyncCircuitFailureThresholdOption      = "AsyncCircuitFailureThreshold"
	AsyncCircuitWindowSecondsOption         = "AsyncCircuitWindowSeconds"
	AsyncCircuitMinimumSamplesOption        = "AsyncCircuitMinimumSamples"
	AsyncCircuitSuccessRateThresholdOption  = "AsyncCircuitSuccessRateThreshold"
	AsyncCircuitInitialOpenSecondsOption    = "AsyncCircuitInitialOpenSeconds"
	AsyncCircuitProbeLeaseSecondsOption     = "AsyncCircuitProbeLeaseSeconds"
	AsyncTaskAttemptRetentionDaysOption     = "AsyncTaskAttemptRetentionDays"
	defaultAsyncFailoverMaxAttempts         = 3
	defaultAsyncPollTransientRetries        = 3
	defaultAsyncCircuitFailureThreshold     = 3
	defaultAsyncCircuitWindowSeconds        = 300
	defaultAsyncCircuitMinimumSamples       = 10
	defaultAsyncCircuitSuccessRateThreshold = 40
	defaultAsyncCircuitInitialOpenSeconds   = 60
	defaultAsyncCircuitProbeLeaseSeconds    = 30
	defaultAsyncTaskAttemptRetentionDays    = 180
	maxAsyncFailoverAttempts                = 3
	maxAsyncPollTransientRetries            = 10
	maxAsyncCircuitWindowSeconds            = 3600
	maxAsyncCircuitInitialOpenSeconds       = 900
	maxAsyncCircuitProbeLeaseSeconds        = 300
	maxAsyncTaskAttemptRetentionDays        = 730
)

type AsyncFailoverSetting struct {
	Enabled                     bool
	MaxAttempts                 int
	PollTransientRetries        int
	CircuitEnabled              bool
	CircuitFailureThreshold     int
	CircuitWindowSeconds        int
	CircuitMinimumSamples       int
	CircuitSuccessRateThreshold int
	CircuitInitialOpenSeconds   int
	CircuitProbeLeaseSeconds    int
	AttemptRetentionDays        int
}

var (
	asyncFailoverSettingMu sync.RWMutex
	asyncFailoverSetting   = AsyncFailoverSetting{
		Enabled:                     false,
		MaxAttempts:                 defaultAsyncFailoverMaxAttempts,
		PollTransientRetries:        defaultAsyncPollTransientRetries,
		CircuitEnabled:              false,
		CircuitFailureThreshold:     defaultAsyncCircuitFailureThreshold,
		CircuitWindowSeconds:        defaultAsyncCircuitWindowSeconds,
		CircuitMinimumSamples:       defaultAsyncCircuitMinimumSamples,
		CircuitSuccessRateThreshold: defaultAsyncCircuitSuccessRateThreshold,
		CircuitInitialOpenSeconds:   defaultAsyncCircuitInitialOpenSeconds,
		CircuitProbeLeaseSeconds:    defaultAsyncCircuitProbeLeaseSeconds,
		AttemptRetentionDays:        defaultAsyncTaskAttemptRetentionDays,
	}
)

func GetAsyncFailoverSetting() AsyncFailoverSetting {
	asyncFailoverSettingMu.RLock()
	defer asyncFailoverSettingMu.RUnlock()
	return asyncFailoverSetting
}

func AsyncFailoverDefaultOptions() map[string]string {
	defaults := AsyncFailoverSetting{
		Enabled:                     false,
		MaxAttempts:                 defaultAsyncFailoverMaxAttempts,
		PollTransientRetries:        defaultAsyncPollTransientRetries,
		CircuitEnabled:              false,
		CircuitFailureThreshold:     defaultAsyncCircuitFailureThreshold,
		CircuitWindowSeconds:        defaultAsyncCircuitWindowSeconds,
		CircuitMinimumSamples:       defaultAsyncCircuitMinimumSamples,
		CircuitSuccessRateThreshold: defaultAsyncCircuitSuccessRateThreshold,
		CircuitInitialOpenSeconds:   defaultAsyncCircuitInitialOpenSeconds,
		CircuitProbeLeaseSeconds:    defaultAsyncCircuitProbeLeaseSeconds,
		AttemptRetentionDays:        defaultAsyncTaskAttemptRetentionDays,
	}
	return map[string]string{
		AsyncFailoverEnabledOption:             strconv.FormatBool(defaults.Enabled),
		AsyncFailoverMaxAttemptsOption:         strconv.Itoa(defaults.MaxAttempts),
		AsyncPollTransientRetriesOption:        strconv.Itoa(defaults.PollTransientRetries),
		AsyncCircuitEnabledOption:              strconv.FormatBool(defaults.CircuitEnabled),
		AsyncCircuitFailureThresholdOption:     strconv.Itoa(defaults.CircuitFailureThreshold),
		AsyncCircuitWindowSecondsOption:        strconv.Itoa(defaults.CircuitWindowSeconds),
		AsyncCircuitMinimumSamplesOption:       strconv.Itoa(defaults.CircuitMinimumSamples),
		AsyncCircuitSuccessRateThresholdOption: strconv.Itoa(defaults.CircuitSuccessRateThreshold),
		AsyncCircuitInitialOpenSecondsOption:   strconv.Itoa(defaults.CircuitInitialOpenSeconds),
		AsyncCircuitProbeLeaseSecondsOption:    strconv.Itoa(defaults.CircuitProbeLeaseSeconds),
		AsyncTaskAttemptRetentionDaysOption:    strconv.Itoa(defaults.AttemptRetentionDays),
	}
}

func IsAsyncFailoverOption(key string) bool {
	_, ok := AsyncFailoverDefaultOptions()[key]
	return ok
}

func ValidateAsyncFailoverOption(key string, value string) error {
	if !IsAsyncFailoverOption(key) {
		return nil
	}
	if key == AsyncFailoverEnabledOption || key == AsyncCircuitEnabledOption {
		if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
		return nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("%s must be an integer", key)
	}
	switch key {
	case AsyncFailoverMaxAttemptsOption:
		return validateAsyncIntegerRange(key, parsed, 1, maxAsyncFailoverAttempts)
	case AsyncPollTransientRetriesOption:
		return validateAsyncIntegerRange(key, parsed, 0, maxAsyncPollTransientRetries)
	case AsyncCircuitFailureThresholdOption:
		return validateAsyncIntegerRange(key, parsed, 1, 100)
	case AsyncCircuitWindowSecondsOption:
		return validateAsyncIntegerRange(key, parsed, 10, maxAsyncCircuitWindowSeconds)
	case AsyncCircuitMinimumSamplesOption:
		return validateAsyncIntegerRange(key, parsed, 1, 10000)
	case AsyncCircuitSuccessRateThresholdOption:
		return validateAsyncIntegerRange(key, parsed, 1, 100)
	case AsyncCircuitInitialOpenSecondsOption:
		return validateAsyncIntegerRange(key, parsed, 1, maxAsyncCircuitInitialOpenSeconds)
	case AsyncCircuitProbeLeaseSecondsOption:
		return validateAsyncIntegerRange(key, parsed, 1, maxAsyncCircuitProbeLeaseSeconds)
	case AsyncTaskAttemptRetentionDaysOption:
		return validateAsyncIntegerRange(key, parsed, 1, maxAsyncTaskAttemptRetentionDays)
	}
	return nil
}

func UpdateAsyncFailoverOption(key string, value string) error {
	if !IsAsyncFailoverOption(key) {
		return nil
	}
	if err := ValidateAsyncFailoverOption(key, value); err != nil {
		return err
	}
	asyncFailoverSettingMu.Lock()
	defer asyncFailoverSettingMu.Unlock()
	switch key {
	case AsyncFailoverEnabledOption:
		asyncFailoverSetting.Enabled, _ = strconv.ParseBool(strings.TrimSpace(value))
	case AsyncFailoverMaxAttemptsOption:
		asyncFailoverSetting.MaxAttempts, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncPollTransientRetriesOption:
		asyncFailoverSetting.PollTransientRetries, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitEnabledOption:
		asyncFailoverSetting.CircuitEnabled, _ = strconv.ParseBool(strings.TrimSpace(value))
	case AsyncCircuitFailureThresholdOption:
		asyncFailoverSetting.CircuitFailureThreshold, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitWindowSecondsOption:
		asyncFailoverSetting.CircuitWindowSeconds, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitMinimumSamplesOption:
		asyncFailoverSetting.CircuitMinimumSamples, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitSuccessRateThresholdOption:
		asyncFailoverSetting.CircuitSuccessRateThreshold, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitInitialOpenSecondsOption:
		asyncFailoverSetting.CircuitInitialOpenSeconds, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncCircuitProbeLeaseSecondsOption:
		asyncFailoverSetting.CircuitProbeLeaseSeconds, _ = strconv.Atoi(strings.TrimSpace(value))
	case AsyncTaskAttemptRetentionDaysOption:
		asyncFailoverSetting.AttemptRetentionDays, _ = strconv.Atoi(strings.TrimSpace(value))
	}
	return nil
}

func validateAsyncIntegerRange(key string, value int, min int, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return nil
}
