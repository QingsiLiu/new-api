package service

import (
	"context"
	"regexp"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

var funnelEvents = map[string]struct{}{
	model.FunnelEventSLPView:        {},
	model.FunnelEventIdentityLink:   {},
	model.FunnelEventAccountActive:  {},
	model.FunnelEventOpenStudio:     {},
	model.FunnelEventPlaygroundFail: {},
}

var funnelFailureCodes = map[string]struct{}{
	"estimate":             {},
	"submit":               {},
	"poll":                 {},
	"task":                 {},
	"timeout":              {},
	"unauthorized":         {},
	"insufficient_balance": {},
	"unknown":              {},
}

var funnelModelSlug = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,94}[a-z0-9])?$`)
var funnelVisitorHMAC = regexp.MustCompile(`^[0-9a-f]{64}$`)

type FunnelInputError struct {
	Status int
	Code   string
}

func (e *FunnelInputError) Error() string { return e.Code }

type FunnelIngestCounters struct {
	Accepted  uint64
	Duplicate uint64
	Rejected  uint64
	Failed    uint64
	Since     int64
}

var (
	funnelAccepted     atomic.Uint64
	funnelDuplicate    atomic.Uint64
	funnelRejected     atomic.Uint64
	funnelFailed       atomic.Uint64
	funnelCounterSince = common.GetTimestamp()
)

func IngestFunnelEvent(ctx context.Context, input model.FunnelEventInput) (model.FunnelIngestResult, error) {
	if err := validateFunnelInput(input); err != nil {
		RecordFunnelRejectedRequest()
		return model.FunnelIngestResult{}, err
	}
	result, err := model.IngestFunnelEventRecord(ctx, input)
	recordFunnelIngestOutcome(result, err)
	return result, err
}

func recordFunnelIngestOutcome(result model.FunnelIngestResult, err error) {
	if err != nil {
		funnelFailed.Add(1)
		return
	}
	if result.Duplicate {
		funnelDuplicate.Add(1)
		return
	}
	funnelAccepted.Add(1)
}

func RecordFunnelRejectedRequest() { funnelRejected.Add(1) }

func GetFunnelIngestCounters() FunnelIngestCounters {
	return FunnelIngestCounters{
		Accepted:  funnelAccepted.Load(),
		Duplicate: funnelDuplicate.Load(),
		Rejected:  funnelRejected.Load(),
		Failed:    funnelFailed.Load(),
		Since:     funnelCounterSince,
	}
}

func validateFunnelInput(input model.FunnelEventInput) error {
	if input.Environment != model.FunnelEnvironmentProduction && input.Environment != model.FunnelEnvironmentStaging {
		return funnelInputError(400, "invalid_environment")
	}
	parsed, err := uuid.Parse(input.EventID)
	if err != nil || parsed.Version() != 4 || parsed.Variant() != uuid.RFC4122 || parsed.String() != input.EventID {
		return funnelInputError(400, "invalid_event_id")
	}
	if input.EventVersion != 1 {
		return funnelInputError(422, "unsupported_event_version")
	}
	if !funnelVisitorHMAC.MatchString(input.VisitorHMAC) {
		return funnelInputError(400, "invalid_visitor_hmac")
	}
	if input.ReceivedAt <= 0 {
		return funnelInputError(400, "invalid_received_at")
	}
	if _, ok := funnelEvents[input.EventName]; !ok {
		return funnelInputError(422, "unsupported_event")
	}

	switch input.EventName {
	case model.FunnelEventSLPView:
		if input.UserID != 0 || input.FailureCode != "" {
			return funnelInputError(422, "forbidden_event_fields")
		}
		if err := validatePublicFields(input); err != nil {
			return err
		}
	case model.FunnelEventPlaygroundFail:
		if input.UserID != 0 {
			return funnelInputError(422, "forbidden_event_fields")
		}
		if err := validatePublicFields(input); err != nil {
			return err
		}
		if _, ok := funnelFailureCodes[input.FailureCode]; !ok {
			return funnelInputError(422, "unsupported_failure_code")
		}
	default:
		if input.UserID <= 0 || input.Locale != "" || input.ModelSlug != "" || input.FailureCode != "" {
			return funnelInputError(422, "invalid_trusted_event_fields")
		}
	}
	return nil
}

func validatePublicFields(input model.FunnelEventInput) error {
	if input.Locale != "zh" && input.Locale != "en" {
		return funnelInputError(400, "invalid_locale")
	}
	if !funnelModelSlug.MatchString(input.ModelSlug) || len(input.ModelSlug) > 96 {
		return funnelInputError(400, "invalid_model_slug")
	}
	return nil
}

func funnelInputError(status int, code string) error {
	return &FunnelInputError{Status: status, Code: code}
}
