package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/go-redis/redis/v8"
)

const (
	asyncCircuitNamespace      = "new-api:async-circuit:v1"
	asyncCircuitRedisTimeout   = time.Second
	asyncCircuitErrorLogPeriod = time.Minute
)

type AsyncCircuitState string

const (
	AsyncCircuitStateClosed   AsyncCircuitState = "closed"
	AsyncCircuitStateOpen     AsyncCircuitState = "open"
	AsyncCircuitStateHalfOpen AsyncCircuitState = "half_open"
	AsyncCircuitStateUnknown  AsyncCircuitState = "unknown"
)

type AsyncCircuitKey struct {
	ChannelID int
	Model     string
	Kind      string
	Action    string
}

type AsyncCircuitDecision struct {
	Allowed           bool              `json:"allowed"`
	State             AsyncCircuitState `json:"state"`
	ProbeToken        string            `json:"-"`
	RetryAfterSeconds int64             `json:"retry_after_seconds,omitempty"`
	Degraded          bool              `json:"degraded,omitempty"`
}

type AsyncCircuitStatus struct {
	Enabled             bool              `json:"enabled"`
	State               AsyncCircuitState `json:"state"`
	Degraded            bool              `json:"degraded,omitempty"`
	ProbeActive         bool              `json:"probe_active,omitempty"`
	RetryAfterSeconds   int64             `json:"retry_after_seconds,omitempty"`
	ConsecutiveFailures int64             `json:"consecutive_failures"`
	WindowSamples       int64             `json:"window_samples"`
	WindowFailures      int64             `json:"window_failures"`
	BackoffLevel        int64             `json:"backoff_level"`
}

type asyncCircuitRedisKeys struct {
	base     string
	state    string
	probe    string
	streak   string
	samples  string
	failures string
	backoff  string
}

const asyncCircuitAcquireScript = `
local state = redis.call("GET", KEYS[1])
if not state then
  return {0, "", 0}
end
local openUntil = tonumber(state)
local now = tonumber(ARGV[1])
if openUntil and openUntil > now then
  return {1, "", openUntil - now}
end
local acquired = redis.call("SET", KEYS[2], ARGV[2], "NX", "PX", ARGV[3])
if acquired then
  return {2, ARGV[2], 0}
end
return {3, "", 0}
`

const asyncCircuitFailureScript = `
local state = redis.call("GET", KEYS[1])
if state then
  local owner = redis.call("GET", KEYS[2])
  if ARGV[8] == "" or owner ~= ARGV[8] then
    return {-1, 0, 0, 0, 0, 0}
  end
end

local now = tonumber(ARGV[1])
local window = tonumber(ARGV[3])
local eventTTL = tonumber(ARGV[11])
redis.call("ZADD", KEYS[4], now, ARGV[2])
redis.call("ZADD", KEYS[5], now, ARGV[2])
redis.call("ZREMRANGEBYSCORE", KEYS[4], "-inf", now - window)
redis.call("ZREMRANGEBYSCORE", KEYS[5], "-inf", now - window)
redis.call("PEXPIRE", KEYS[4], eventTTL)
redis.call("PEXPIRE", KEYS[5], eventTTL)

local streak = redis.call("INCR", KEYS[3])
redis.call("PEXPIRE", KEYS[3], eventTTL)
local samples = redis.call("ZCARD", KEYS[4])
local failures = redis.call("ZCARD", KEYS[5])
local successes = samples - failures
local successRate = 100
if samples > 0 then
  successRate = math.floor((successes * 100) / samples)
end

local shouldOpen = tonumber(ARGV[9]) == 1
if streak >= tonumber(ARGV[4]) then
  shouldOpen = true
end
if samples >= tonumber(ARGV[5]) and successRate < tonumber(ARGV[6]) then
  shouldOpen = true
end

local level = tonumber(redis.call("GET", KEYS[6]) or "0")
if state then
  if level < 1 then level = 1 end
  level = level + 1
  if level > 3 then level = 3 end
  shouldOpen = true
elseif shouldOpen and level < 1 then
  level = 1
end

if shouldOpen then
  local multiplier = 1
  if level == 2 then multiplier = 5 end
  if level >= 3 then multiplier = 15 end
  local openFor = tonumber(ARGV[7]) * multiplier
  local openUntil = now + openFor
  redis.call("SET", KEYS[1], tostring(openUntil), "PX", openFor + tonumber(ARGV[10]))
  redis.call("SET", KEYS[6], tostring(level), "PX", ARGV[12])
  redis.call("DEL", KEYS[2])
  return {1, streak, samples, failures, level, openFor}
end

return {0, streak, samples, failures, level, 0}
`

const asyncCircuitSuccessScript = `
local state = redis.call("GET", KEYS[1])
if state then
  local owner = redis.call("GET", KEYS[2])
  if ARGV[5] == "" or owner ~= ARGV[5] then
    return 0
  end
  redis.call("DEL", KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5], KEYS[6])
  return 1
end

local now = tonumber(ARGV[1])
local window = tonumber(ARGV[3])
redis.call("ZADD", KEYS[4], now, ARGV[2])
redis.call("ZREMRANGEBYSCORE", KEYS[4], "-inf", now - window)
redis.call("ZREMRANGEBYSCORE", KEYS[5], "-inf", now - window)
redis.call("PEXPIRE", KEYS[4], ARGV[4])
if redis.call("EXISTS", KEYS[5]) == 1 then
  redis.call("PEXPIRE", KEYS[5], ARGV[4])
end
redis.call("DEL", KEYS[2], KEYS[3], KEYS[6])
return 2
`

var (
	asyncCircuitNow              = time.Now
	asyncCircuitEventSequence    atomic.Uint64
	asyncCircuitRedisErrorLogMu  sync.Mutex
	asyncCircuitRedisErrorLogged time.Time
)

// AcquireAsyncCircuit returns whether an async attempt may use this
// channel/model/operation. Redis outages are deliberately fail-open.
func AcquireAsyncCircuit(key AsyncCircuitKey) AsyncCircuitDecision {
	setting := operation_setting.GetAsyncFailoverSetting()
	if !setting.CircuitEnabled {
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateClosed}
	}
	keys, ok := buildAsyncCircuitRedisKeys(key)
	if !ok || !asyncCircuitRedisReady() {
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateUnknown, Degraded: true}
	}
	probeToken, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateUnknown, Degraded: true}
	}
	nowMillis := asyncCircuitNow().UnixMilli()
	leaseMillis := int64(setting.CircuitProbeLeaseSeconds) * 1000
	ctx, cancel := asyncCircuitRedisContext()
	defer cancel()
	result, err := common.RDB.Eval(ctx, asyncCircuitAcquireScript, []string{keys.state, keys.probe}, nowMillis, probeToken, leaseMillis).Result()
	if err != nil {
		logAsyncCircuitRedisError("acquire", err)
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateUnknown, Degraded: true}
	}
	values, ok := result.([]interface{})
	if !ok || len(values) < 3 {
		logAsyncCircuitRedisError("acquire_decode", fmt.Errorf("unexpected result type %T", result))
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateUnknown, Degraded: true}
	}
	code := asyncCircuitResultInt64(values[0])
	switch code {
	case 0:
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateClosed}
	case 1:
		return AsyncCircuitDecision{
			Allowed:           false,
			State:             AsyncCircuitStateOpen,
			RetryAfterSeconds: asyncCircuitMillisToSeconds(asyncCircuitResultInt64(values[2])),
		}
	case 2:
		return AsyncCircuitDecision{
			Allowed:    true,
			State:      AsyncCircuitStateHalfOpen,
			ProbeToken: asyncCircuitResultString(values[1]),
		}
	case 3:
		return AsyncCircuitDecision{Allowed: false, State: AsyncCircuitStateHalfOpen}
	default:
		return AsyncCircuitDecision{Allowed: true, State: AsyncCircuitStateUnknown, Degraded: true}
	}
}

// RecordAsyncCircuitSuccess records a provider-attributable successful
// attempt. When the circuit is half-open, only the probe owner may close it.
func RecordAsyncCircuitSuccess(key AsyncCircuitKey, probeToken string) AsyncCircuitStatus {
	setting := operation_setting.GetAsyncFailoverSetting()
	if !setting.CircuitEnabled {
		return AsyncCircuitStatus{Enabled: false, State: AsyncCircuitStateClosed}
	}
	keys, ok := buildAsyncCircuitRedisKeys(key)
	if !ok || !asyncCircuitRedisReady() {
		return degradedAsyncCircuitStatus(true)
	}
	nowMillis := asyncCircuitNow().UnixMilli()
	windowMillis := int64(setting.CircuitWindowSeconds) * 1000
	eventTTLMillis := windowMillis * 2
	ctx, cancel := asyncCircuitRedisContext()
	defer cancel()
	_, err := common.RDB.Eval(ctx, asyncCircuitSuccessScript, keys.all(),
		nowMillis,
		asyncCircuitEventMember(nowMillis),
		windowMillis,
		eventTTLMillis,
		probeToken,
	).Result()
	if err != nil {
		logAsyncCircuitRedisError("success", err)
		return degradedAsyncCircuitStatus(true)
	}
	return GetAsyncCircuitStatus(key)
}

// RecordAsyncCircuitFailure records a provider-attributable failure. Set
// immediateOpen for persistent failures such as invalid credentials, exhausted
// upstream balance, or a missing upstream model.
func RecordAsyncCircuitFailure(key AsyncCircuitKey, probeToken string, immediateOpen bool) AsyncCircuitStatus {
	setting := operation_setting.GetAsyncFailoverSetting()
	if !setting.CircuitEnabled {
		return AsyncCircuitStatus{Enabled: false, State: AsyncCircuitStateClosed}
	}
	keys, ok := buildAsyncCircuitRedisKeys(key)
	if !ok || !asyncCircuitRedisReady() {
		return degradedAsyncCircuitStatus(true)
	}
	nowMillis := asyncCircuitNow().UnixMilli()
	windowMillis := int64(setting.CircuitWindowSeconds) * 1000
	initialOpenMillis := int64(setting.CircuitInitialOpenSeconds) * 1000
	probeLeaseMillis := int64(setting.CircuitProbeLeaseSeconds) * 1000
	eventTTLMillis := windowMillis * 2
	stateExtraMillis := maxInt64(windowMillis, probeLeaseMillis*2)
	backoffTTLMillis := maxInt64(eventTTLMillis, initialOpenMillis*15+windowMillis)
	immediate := 0
	if immediateOpen {
		immediate = 1
	}
	ctx, cancel := asyncCircuitRedisContext()
	defer cancel()
	_, err := common.RDB.Eval(ctx, asyncCircuitFailureScript, keys.all(),
		nowMillis,
		asyncCircuitEventMember(nowMillis),
		windowMillis,
		setting.CircuitFailureThreshold,
		setting.CircuitMinimumSamples,
		setting.CircuitSuccessRateThreshold,
		initialOpenMillis,
		probeToken,
		immediate,
		stateExtraMillis,
		eventTTLMillis,
		backoffTTLMillis,
	).Result()
	if err != nil {
		logAsyncCircuitRedisError("failure", err)
		return degradedAsyncCircuitStatus(true)
	}
	return GetAsyncCircuitStatus(key)
}

// GetAsyncCircuitStatus reads the hot circuit state without changing it.
func GetAsyncCircuitStatus(key AsyncCircuitKey) AsyncCircuitStatus {
	setting := operation_setting.GetAsyncFailoverSetting()
	if !setting.CircuitEnabled {
		return AsyncCircuitStatus{Enabled: false, State: AsyncCircuitStateClosed}
	}
	keys, ok := buildAsyncCircuitRedisKeys(key)
	if !ok || !asyncCircuitRedisReady() {
		return degradedAsyncCircuitStatus(true)
	}
	ctx, cancel := asyncCircuitRedisContext()
	defer cancel()
	pipe := common.RDB.Pipeline()
	windowStartMillis := asyncCircuitNow().UnixMilli() - int64(setting.CircuitWindowSeconds)*1000
	stateCmd := pipe.Get(ctx, keys.state)
	probeCmd := pipe.Exists(ctx, keys.probe)
	streakCmd := pipe.Get(ctx, keys.streak)
	pipe.ZRemRangeByScore(ctx, keys.samples, "-inf", strconv.FormatInt(windowStartMillis, 10))
	pipe.ZRemRangeByScore(ctx, keys.failures, "-inf", strconv.FormatInt(windowStartMillis, 10))
	sampleCmd := pipe.ZCard(ctx, keys.samples)
	failureCmd := pipe.ZCard(ctx, keys.failures)
	backoffCmd := pipe.Get(ctx, keys.backoff)
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		logAsyncCircuitRedisError("status", err)
		return degradedAsyncCircuitStatus(true)
	}

	status := AsyncCircuitStatus{
		Enabled:             true,
		State:               AsyncCircuitStateClosed,
		ProbeActive:         probeCmd.Val() > 0,
		ConsecutiveFailures: asyncCircuitStringInt64(streakCmd.Val()),
		WindowSamples:       sampleCmd.Val(),
		WindowFailures:      failureCmd.Val(),
		BackoffLevel:        asyncCircuitStringInt64(backoffCmd.Val()),
	}
	if stateCmd.Err() == nil {
		openUntil := asyncCircuitStringInt64(stateCmd.Val())
		remainingMillis := openUntil - asyncCircuitNow().UnixMilli()
		if remainingMillis > 0 {
			status.State = AsyncCircuitStateOpen
			status.RetryAfterSeconds = asyncCircuitMillisToSeconds(remainingMillis)
		} else {
			status.State = AsyncCircuitStateHalfOpen
		}
	}
	return status
}

// ResetAsyncCircuitByChannel clears all model/operation circuits for a channel.
// It never mutates channels.status or abilities.
func ResetAsyncCircuitByChannel(channelID int) (int64, error) {
	if channelID <= 0 || !asyncCircuitRedisReady() {
		return 0, nil
	}
	prefix := asyncCircuitChannelPrefix(channelID)
	ctx, cancel := asyncCircuitRedisContext()
	defer cancel()
	var cursor uint64
	var deleted int64
	for {
		keys, next, err := common.RDB.Scan(ctx, cursor, prefix+"*", 256).Result()
		if err != nil {
			return deleted, err
		}
		if len(keys) > 0 {
			count, err := common.RDB.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, err
			}
			deleted += count
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return deleted, nil
}

func buildAsyncCircuitRedisKeys(key AsyncCircuitKey) (asyncCircuitRedisKeys, bool) {
	if key.ChannelID <= 0 {
		return asyncCircuitRedisKeys{}, false
	}
	canonical := strings.ToLower(strings.TrimSpace(key.Model)) + "\x00" +
		strings.ToLower(strings.TrimSpace(key.Kind)) + "\x00" +
		strings.ToLower(strings.TrimSpace(key.Action))
	if strings.Trim(canonical, "\x00") == "" {
		return asyncCircuitRedisKeys{}, false
	}
	digest := sha256.Sum256([]byte(canonical))
	base := asyncCircuitChannelPrefix(key.ChannelID) + hex.EncodeToString(digest[:])
	return asyncCircuitRedisKeys{
		base:     base,
		state:    base + ":state",
		probe:    base + ":probe",
		streak:   base + ":streak",
		samples:  base + ":samples",
		failures: base + ":failures",
		backoff:  base + ":backoff",
	}, true
}

func asyncCircuitChannelPrefix(channelID int) string {
	return asyncCircuitNamespace + ":" + strconv.Itoa(channelID) + ":"
}

func (keys asyncCircuitRedisKeys) all() []string {
	return []string{keys.state, keys.probe, keys.streak, keys.samples, keys.failures, keys.backoff}
}

func asyncCircuitRedisReady() bool {
	return common.RedisEnabled && common.RDB != nil
}

func asyncCircuitRedisContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), asyncCircuitRedisTimeout)
}

func asyncCircuitEventMember(nowMillis int64) string {
	sequence := asyncCircuitEventSequence.Add(1)
	random, err := common.GenerateRandomCharsKey(12)
	if err != nil {
		random = "fallback"
	}
	return fmt.Sprintf("%d:%d:%s", nowMillis, sequence, random)
}

func asyncCircuitResultInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

func asyncCircuitResultString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(value)
	}
}

func asyncCircuitStringInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func asyncCircuitMillisToSeconds(milliseconds int64) int64 {
	if milliseconds <= 0 {
		return 0
	}
	return (milliseconds + 999) / 1000
}

func degradedAsyncCircuitStatus(enabled bool) AsyncCircuitStatus {
	return AsyncCircuitStatus{Enabled: enabled, State: AsyncCircuitStateUnknown, Degraded: true}
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func logAsyncCircuitRedisError(operation string, err error) {
	if err == nil {
		return
	}
	now := time.Now()
	asyncCircuitRedisErrorLogMu.Lock()
	defer asyncCircuitRedisErrorLogMu.Unlock()
	if !asyncCircuitRedisErrorLogged.IsZero() && now.Sub(asyncCircuitRedisErrorLogged) < asyncCircuitErrorLogPeriod {
		return
	}
	asyncCircuitRedisErrorLogged = now
	common.SysError(fmt.Sprintf("async circuit Redis operation failed: operation=%s error=%v", operation, err))
}
