package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestAsyncAttemptErrorClassifiesCapabilityBeforeInvalidRequest(t *testing.T) {
	err := classifyAsyncAttemptError(
		errors.New(`invalid_request_error: Unknown parameter: 'response_format'`),
		model.AsyncAttemptStageSubmit,
		model.AsyncAttemptAcceptanceNotAccepted,
	)
	require.Equal(t, asyncFailureClassCapability, err.FailureClass)
	require.True(t, err.Retryable)
}

func TestAsyncAttemptErrorKeepsUserParameterFailureNonRetryable(t *testing.T) {
	err := classifyAsyncAttemptError(
		errors.New("invalid parameter: duration"),
		model.AsyncAttemptStageSubmit,
		model.AsyncAttemptAcceptanceNotAccepted,
	)
	require.Equal(t, asyncFailureClassParameter, err.FailureClass)
	require.False(t, err.Retryable)
}

func TestAsyncAttemptUnknownAcceptanceRequiresIdempotentChannel(t *testing.T) {
	err := classifyAsyncAttemptError(
		context.DeadlineExceeded,
		model.AsyncAttemptStageSubmit,
		model.AsyncAttemptAcceptanceUnknown,
	)
	require.True(t, err.Retryable)

	conservative := &model.Channel{}
	require.False(t, asyncAttemptCanFailover(err, conservative))

	settingJSON := `{"async_failover":{"unknown_submit":"idempotent"}}`
	idempotent := &model.Channel{Setting: &settingJSON}
	require.True(t, asyncAttemptCanFailover(err, idempotent))
}

func TestAsyncAttemptConnectionRefusedIsKnownNotAccepted(t *testing.T) {
	err := classifyAsyncAttemptError(
		&testNetError{message: "dial tcp: connection refused"},
		model.AsyncAttemptStageSubmit,
		model.AsyncAttemptAcceptanceUnknown,
	)
	require.Equal(t, asyncFailureClassNetwork, err.FailureClass)
	require.Equal(t, model.AsyncAttemptAcceptanceNotAccepted, err.AcceptanceState)
	require.True(t, err.Retryable)
}

func TestAsyncProviderCodeExtractionKeepsOnlyShortStructuredCode(t *testing.T) {
	require.Equal(t, "insufficient_quota", asyncProviderCodeFromBody([]byte(`{"error":{"code":"insufficient_quota","message":"secret detail"}}`)))
	require.Equal(t, "upstream_error", asyncProviderCodeFromBody([]byte(`{"type":"upstream_error","message":"detail"}`)))
	require.Empty(t, asyncProviderCodeFromBody([]byte(`not-json`)))
}

type testNetError struct {
	message string
}

func (e *testNetError) Error() string   { return e.message }
func (e *testNetError) Timeout() bool   { return false }
func (e *testNetError) Temporary() bool { return true }
