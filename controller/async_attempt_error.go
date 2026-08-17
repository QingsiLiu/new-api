package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	asyncFailureClassAuthentication = "authentication"
	asyncFailureClassUpstreamQuota  = "upstream_quota"
	asyncFailureClassRateLimit      = "rate_limit"
	asyncFailureClassCapacity       = "capacity"
	asyncFailureClassModel          = "model_unavailable"
	asyncFailureClassParameter      = "parameter"
	asyncFailureClassContentPolicy  = "content_policy"
	asyncFailureClassNetwork        = "network"
	asyncFailureClassTimeout        = "timeout"
	asyncFailureClassCapability     = "capability"
	asyncFailureClassCanceled       = "canceled"
	asyncFailureClassLocal          = "local"
	asyncFailureClassUnknown        = "unknown"
)

type asyncAttemptError struct {
	Err             error
	Stage           string
	FailureClass    string
	AcceptanceState string
	HTTPStatus      int
	ProviderCode    string
	Retryable       bool
}

func (e *asyncAttemptError) Error() string {
	if e == nil || e.Err == nil {
		return "async attempt failed"
	}
	return e.Err.Error()
}

func (e *asyncAttemptError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newAsyncHTTPAttemptError(stage string, status int, providerCode string, message string, acceptance string) error {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	attemptErr := classifyAsyncAttemptError(errors.New(message), stage, acceptance)
	attemptErr.HTTPStatus = status
	attemptErr.ProviderCode = truncateAsyncProviderCode(providerCode)
	class, retryable := asyncHTTPFailureClass(status, message)
	attemptErr.FailureClass = class
	attemptErr.Retryable = retryable
	return attemptErr
}

func classifyAsyncAttemptError(err error, stage string, acceptance string) *asyncAttemptError {
	if err == nil {
		return nil
	}
	var existing *asyncAttemptError
	if errors.As(err, &existing) {
		if existing.Stage == "" {
			existing.Stage = stage
		}
		if existing.AcceptanceState == "" {
			existing.AcceptanceState = acceptance
		}
		return existing
	}
	result := &asyncAttemptError{
		Err:             err,
		Stage:           stage,
		FailureClass:    asyncFailureClassUnknown,
		AcceptanceState: acceptance,
	}
	if result.AcceptanceState == "" {
		result.AcceptanceState = model.AsyncAttemptAcceptanceNotAccepted
	}
	if errors.Is(err, context.Canceled) {
		result.FailureClass = asyncFailureClassCanceled
		return result
	}
	if errors.Is(err, context.DeadlineExceeded) {
		result.FailureClass = asyncFailureClassTimeout
		result.AcceptanceState = model.AsyncAttemptAcceptanceUnknown
		result.Retryable = true
		return result
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			result.FailureClass = asyncFailureClassTimeout
			result.AcceptanceState = model.AsyncAttemptAcceptanceUnknown
			result.Retryable = true
			return result
		}
		result.FailureClass = asyncFailureClassNetwork
		result.Retryable = true
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "connection refused") ||
			strings.Contains(message, "no such host") ||
			strings.Contains(message, "tls handshake") {
			result.AcceptanceState = model.AsyncAttemptAcceptanceNotAccepted
		} else {
			result.AcceptanceState = model.AsyncAttemptAcceptanceUnknown
		}
		return result
	}
	result.FailureClass, result.Retryable = asyncMessageFailureClass(err.Error())
	return result
}

func asyncHTTPFailureClass(status int, message string) (string, bool) {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return asyncFailureClassAuthentication, true
	}
	if status == http.StatusTooManyRequests {
		return asyncFailureClassRateLimit, true
	}
	if status == http.StatusNotFound || status == http.StatusConflict {
		return asyncFailureClassModel, true
	}
	if status >= 500 {
		return asyncFailureClassCapacity, true
	}
	messageClass, messageRetryable := asyncMessageFailureClass(message)
	if messageClass != asyncFailureClassUnknown {
		return messageClass, messageRetryable
	}
	if status == http.StatusRequestTimeout {
		return asyncFailureClassTimeout, true
	}
	if status >= 400 && status < 500 {
		return asyncFailureClassParameter, false
	}
	return asyncFailureClassUnknown, false
}

func asyncMessageFailureClass(message string) (string, bool) {
	lower := strings.ToLower(message)
	switch {
	case containsAnyAsyncText(lower, "content policy", "safety policy", "prohibited content", "违反了我们的内容政策", "内容审核"):
		return asyncFailureClassContentPolicy, false
	case containsAnyAsyncText(lower, "unknown parameter", "unknown_param", "response_format", "not supported by channel"):
		return asyncFailureClassCapability, true
	case containsAnyAsyncText(lower, "unsupported action", "invalid parameter", "invalid_request", "请求包含非法参数", "content is required"):
		return asyncFailureClassParameter, false
	case containsAnyAsyncText(lower, "insufficient quota", "insufficient_user_quota", "credit balance", "额度不足", "no available image quota"):
		return asyncFailureClassUpstreamQuota, true
	case containsAnyAsyncText(lower, "unauthorized", "permission denied", "invalid api key", "authentication"):
		return asyncFailureClassAuthentication, true
	case containsAnyAsyncText(lower, "rate limit", "too many requests"):
		return asyncFailureClassRateLimit, true
	case containsAnyAsyncText(lower, "model not found", "model unavailable", "unsupported model"):
		return asyncFailureClassModel, true
	case containsAnyAsyncText(lower, "temporarily unavailable", "upstream_error", "overloaded", "capacity", "service unavailable"):
		return asyncFailureClassCapacity, true
	case containsAnyAsyncText(lower, "context canceled", "canceled", "cancelled"):
		return asyncFailureClassCanceled, false
	case containsAnyAsyncText(lower, "archive", "download output", "inline base64", "invalid data url"):
		return asyncFailureClassLocal, false
	default:
		return asyncFailureClassUnknown, false
	}
}

func containsAnyAsyncText(message string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(message, value) {
			return true
		}
	}
	return false
}

func truncateAsyncProviderCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func asyncProviderCodeFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := common.Unmarshal(body, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"code", "type", "status"} {
		if value := strings.TrimSpace(fmt.Sprint(payload[key])); value != "" && value != "<nil>" && value != "0" {
			return truncateAsyncProviderCode(value)
		}
	}
	if nested, ok := payload["error"].(map[string]interface{}); ok {
		for _, key := range []string{"code", "type", "status"} {
			if value := strings.TrimSpace(fmt.Sprint(nested[key])); value != "" && value != "<nil>" && value != "0" {
				return truncateAsyncProviderCode(value)
			}
		}
	}
	return ""
}

func asyncAttemptCanFailover(err *asyncAttemptError, channel *model.Channel) bool {
	if err == nil || !err.Retryable || err.FailureClass == asyncFailureClassCanceled || err.FailureClass == asyncFailureClassLocal {
		return false
	}
	if err.AcceptanceState != model.AsyncAttemptAcceptanceUnknown {
		return true
	}
	if channel == nil {
		return false
	}
	return channel.GetSetting().AsyncFailover.UnknownSubmit == dto.AsyncFailoverUnknownSubmitIdempotent
}

type asyncAttemptTelemetry struct {
	StartedAt      time.Time
	SubmittedAt    time.Time
	UpstreamTaskID string
	PollCount      int
	PollErrorCount int
}

type asyncAttemptContextKey struct{}

type asyncAttemptContextValue struct {
	Key       string
	Telemetry *asyncAttemptTelemetry
}

func withAsyncAttemptContext(parent context.Context, attemptKey string) (context.Context, *asyncAttemptTelemetry) {
	telemetry := &asyncAttemptTelemetry{StartedAt: time.Now()}
	return context.WithValue(parent, asyncAttemptContextKey{}, asyncAttemptContextValue{Key: attemptKey, Telemetry: telemetry}), telemetry
}

func asyncAttemptContext(ctx context.Context) asyncAttemptContextValue {
	if ctx == nil {
		return asyncAttemptContextValue{}
	}
	value, _ := ctx.Value(asyncAttemptContextKey{}).(asyncAttemptContextValue)
	return value
}

func markAsyncAttemptSubmitted(ctx context.Context) {
	value := asyncAttemptContext(ctx)
	if value.Telemetry != nil && value.Telemetry.SubmittedAt.IsZero() {
		value.Telemetry.SubmittedAt = time.Now()
	}
}

func markAsyncAttemptUpstreamTask(ctx context.Context, taskID string) {
	value := asyncAttemptContext(ctx)
	if value.Telemetry != nil {
		value.Telemetry.UpstreamTaskID = strings.TrimSpace(taskID)
	}
}

func markAsyncAttemptPoll(ctx context.Context, failed bool) {
	value := asyncAttemptContext(ctx)
	if value.Telemetry == nil {
		return
	}
	value.Telemetry.PollCount++
	if failed {
		value.Telemetry.PollErrorCount++
	}
}

func applyAsyncAttemptIdempotencyHeader(ctx context.Context, channel *model.Channel, request *http.Request) {
	if channel == nil || request == nil || channel.GetSetting().AsyncFailover.UnknownSubmit != dto.AsyncFailoverUnknownSubmitIdempotent {
		return
	}
	if key := strings.TrimSpace(asyncAttemptContext(ctx).Key); key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
}

func asyncAttemptKey(taskID string, attemptNo int) string {
	return fmt.Sprintf("%s-attempt-%d", taskID, attemptNo)
}

func asyncPollWithTransientRetry[T any](ctx context.Context, poll func() (T, error)) (T, error) {
	var zero T
	if poll == nil {
		return zero, errors.New("poll function is required")
	}
	budget := operation_setting.GetAsyncFailoverSetting().PollTransientRetries
	for retry := 0; ; retry++ {
		result, err := poll()
		if err == nil {
			return result, nil
		}
		attemptErr := classifyAsyncAttemptError(err, model.AsyncAttemptStagePoll, model.AsyncAttemptAcceptanceAccepted)
		if !asyncPollErrorIsTransient(attemptErr) || retry >= budget {
			// Poll transport exhaustion is intentionally not eligible for a new
			// upstream task: the accepted task may still be running.
			attemptErr.Retryable = false
			return zero, attemptErr
		}
		delay := time.Second << min(retry, 2)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func asyncPollErrorIsTransient(err *asyncAttemptError) bool {
	if err == nil {
		return false
	}
	switch err.FailureClass {
	case asyncFailureClassNetwork, asyncFailureClassTimeout, asyncFailureClassRateLimit, asyncFailureClassCapacity:
		return true
	default:
		return false
	}
}
