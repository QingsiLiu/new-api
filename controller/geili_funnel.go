package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var geiliFunnelCurrentTime = time.Now

var geiliFunnelEventKeys = map[string]struct{}{
	"event_id": {}, "event": {}, "version": {}, "environment": {}, "visitor_hmac": {},
	"locale": {}, "model": {}, "failure_code": {}, "user_id": {},
}

func IngestGeiliFunnelEvent(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rejectGeiliFunnelDTO(c, http.StatusBadRequest)
		return
	}
	var raw map[string]json.RawMessage
	if err := common.Unmarshal(body, &raw); err != nil || raw == nil {
		rejectGeiliFunnelDTO(c, http.StatusBadRequest)
		return
	}
	for key := range raw {
		if _, ok := geiliFunnelEventKeys[key]; !ok {
			rejectGeiliFunnelDTO(c, http.StatusBadRequest)
			return
		}
	}
	for _, key := range []string{"event_id", "event", "version", "environment", "visitor_hmac"} {
		if _, ok := raw[key]; !ok {
			rejectGeiliFunnelDTO(c, http.StatusBadRequest)
			return
		}
	}

	var request dto.GeiliFunnelEventRequest
	if err := common.Unmarshal(body, &request); err != nil {
		rejectGeiliFunnelDTO(c, http.StatusBadRequest)
		return
	}
	if status := validateGeiliFunnelKeyShape(request.Event, raw); status != 0 {
		rejectGeiliFunnelDTO(c, status)
		return
	}

	_, err = service.IngestFunnelEvent(c.Request.Context(), model.FunnelEventInput{
		Environment:  request.Environment,
		EventID:      request.EventID,
		EventName:    request.Event,
		EventVersion: request.Version,
		VisitorHMAC:  request.VisitorHMAC,
		Locale:       request.Locale,
		ModelSlug:    request.Model,
		FailureCode:  request.FailureCode,
		UserID:       request.UserID,
		ReceivedAt:   common.GetTimestamp(),
	})
	if err == nil {
		c.Status(http.StatusNoContent)
		return
	}
	var inputErr *service.FunnelInputError
	if errors.As(err, &inputErr) {
		c.AbortWithStatus(inputErr.Status)
		return
	}
	common.SysError("geili_funnel_ingest_failed")
	c.AbortWithStatus(http.StatusServiceUnavailable)
}

func GetGeiliFunnelSummary(c *gin.Context) {
	now := geiliFunnelCurrentTime().UTC()
	query, err := parseGeiliFunnelSummaryQuery(c.Request.URL.Query(), now)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	response, err := service.QueryGeiliFunnelSummary(c.Request.Context(), query)
	if err != nil {
		common.SysError("geili_funnel_summary_failed")
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.JSON(http.StatusOK, response)
}

func parseGeiliFunnelSummaryQuery(values url.Values, now time.Time) (service.FunnelSummaryQuery, error) {
	allowed := map[string]struct{}{"from": {}, "to": {}, "dimension": {}, "environment": {}}
	for key, entries := range values {
		if _, ok := allowed[key]; !ok || len(entries) != 1 {
			return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
		}
	}

	today := now.UTC().Truncate(24 * time.Hour)
	fromDay := today.AddDate(0, 0, -29)
	toDay := today
	fromValue, hasFrom := values["from"]
	toValue, hasTo := values["to"]
	if hasFrom != hasTo {
		return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
	}
	if hasFrom {
		var err error
		fromDay, err = time.Parse("2006-01-02", fromValue[0])
		if err != nil {
			return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
		}
		toDay, err = time.Parse("2006-01-02", toValue[0])
		if err != nil {
			return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
		}
	}
	if fromDay.After(toDay) || fromDay.After(today) || toDay.After(today) {
		return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
	}
	if inclusiveDays := int(toDay.Sub(fromDay)/(24*time.Hour)) + 1; inclusiveDays < 1 || inclusiveDays > 730 {
		return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
	}

	dimension := values.Get("dimension")
	if dimension == "" {
		dimension = "all"
	}
	environment := values.Get("environment")
	if environment == "" {
		environment = model.FunnelEnvironmentProduction
	}
	query := service.FunnelSummaryQuery{
		Environment: environment,
		Dimension:   dimension,
		From:        fromDay.Unix(),
		To:          toDay.AddDate(0, 0, 1).Unix(),
		Now:         now.Unix(),
	}
	if query.Environment != model.FunnelEnvironmentProduction && query.Environment != model.FunnelEnvironmentStaging {
		return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
	}
	if query.Dimension != "all" && query.Dimension != "locale" && query.Dimension != "model" {
		return service.FunnelSummaryQuery{}, service.ErrInvalidFunnelSummaryQuery
	}
	return query, nil
}

func validateGeiliFunnelKeyShape(event string, raw map[string]json.RawMessage) int {
	_, hasLocale := raw["locale"]
	_, hasModel := raw["model"]
	_, hasFailure := raw["failure_code"]
	_, hasUser := raw["user_id"]
	switch event {
	case model.FunnelEventSLPView:
		if !hasLocale || !hasModel || hasFailure || hasUser {
			return http.StatusUnprocessableEntity
		}
	case model.FunnelEventPlaygroundFail:
		if !hasLocale || !hasModel || !hasFailure || hasUser {
			return http.StatusUnprocessableEntity
		}
	case model.FunnelEventIdentityLink, model.FunnelEventAccountActive, model.FunnelEventOpenStudio:
		if hasLocale || hasModel || hasFailure || !hasUser {
			return http.StatusUnprocessableEntity
		}
	}
	return 0
}

func rejectGeiliFunnelDTO(c *gin.Context, status int) {
	service.RecordFunnelRejectedRequest()
	c.AbortWithStatus(status)
}
