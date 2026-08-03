package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

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
