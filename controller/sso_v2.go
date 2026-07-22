package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ssoV2EnabledEnv      = "SSO_V2_ENABLED"
	ssoStudioAudience    = "studio"
	ssoTicketRedisPrefix = "sso_ticket_v2:"
	ssoTicketTTL         = 60 * time.Second
)

type ssoTicketPayloadV2 struct {
	UserID    int    `json:"user_id"`
	Audience  string `json:"audience"`
	ExpiresAt int64  `json:"expires_at"`
	Nonce     string `json:"nonce"`
}

type SSOExchangeV2Request struct {
	Ticket   string `json:"ticket" binding:"required"`
	Audience string `json:"audience" binding:"required"`
}

type SSOExchangeV2Response struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	UserID    int    `json:"user_id,omitempty"`
	Audience  string `json:"audience,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func ssoV2Enabled() bool {
	return common.GetEnvOrDefaultBool(ssoV2EnabledEnv, false)
}

// SSORedirectV2 issues a short-lived, audience-bound opaque ticket. The Redis
// value deliberately contains no account credential; Studio resolves the user
// server-to-server after the exchange.
func SSORedirectV2(c *gin.Context) {
	if !ssoV2Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "SSO v2 is not enabled"})
		return
	}
	redirectURI, err := validateSSOV2RedirectURI(strings.TrimSpace(c.Query("redirect_uri")))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "redirect_uri not allowed"})
		return
	}
	audience := strings.TrimSpace(c.Query("audience"))
	if audience != ssoStudioAudience {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "audience is not supported"})
		return
	}

	userID := c.GetInt("id")
	if userID == 0 {
		query := redirectURI.Query()
		query.Set("sso", "none")
		redirectURI.RawQuery = query.Encode()
		c.Redirect(http.StatusFound, redirectURI.String())
		return
	}
	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "user is not active"})
		return
	}
	if common.RDB == nil || !common.RedisEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "SSO temporarily unavailable"})
		return
	}

	ticket, err := common.GenerateRandomKey(32)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	nonce, err := common.GenerateRandomKey(24)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiresAt := time.Now().UTC().Add(ssoTicketTTL)
	payload, err := common.Marshal(ssoTicketPayloadV2{
		UserID:    userID,
		Audience:  audience,
		ExpiresAt: expiresAt.Unix(),
		Nonce:     nonce,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := common.RedisSet(ssoTicketRedisPrefix+ticket, string(payload), ssoTicketTTL); err != nil {
		common.SysLog(fmt.Sprintf("sso_v2 issue failed: audience=%s user_id=%d error_type=%T", audience, userID, err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "SSO temporarily unavailable"})
		return
	}
	logSSOV2Event("issued", ticket, audience, userID)

	query := redirectURI.Query()
	query.Set("ticket", ticket)
	query.Set("audience", audience)
	query.Del("code")
	redirectURI.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, redirectURI.String())
}

// SSOExchangeV2 atomically consumes a ticket and returns only the New API user
// id. A mismatched audience intentionally burns the ticket to prevent probing.
func SSOExchangeV2(c *gin.Context) {
	if !ssoV2Enabled() {
		c.JSON(http.StatusNotFound, SSOExchangeV2Response{Success: false, Message: "SSO v2 is not enabled"})
		return
	}
	var req SSOExchangeV2Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, SSOExchangeV2Response{Success: false, Message: "ticket and audience are required"})
		return
	}
	req.Ticket = strings.TrimSpace(req.Ticket)
	req.Audience = strings.TrimSpace(req.Audience)
	if len(req.Ticket) < 16 || len(req.Ticket) > 512 || req.Audience == "" {
		c.JSON(http.StatusBadRequest, SSOExchangeV2Response{Success: false, Message: "ticket and audience are required"})
		return
	}
	if common.RDB == nil || !common.RedisEnabled {
		c.JSON(http.StatusServiceUnavailable, SSOExchangeV2Response{Success: false, Message: "SSO temporarily unavailable"})
		return
	}

	payloadJSON, err := common.RedisGetDel(ssoTicketRedisPrefix + req.Ticket)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.JSON(http.StatusUnauthorized, SSOExchangeV2Response{Success: false, Message: "invalid or expired ticket"})
			return
		}
		common.SysLog(fmt.Sprintf("sso_v2 exchange failed: ticket_hash=%s error_type=%T", ssoTicketHash(req.Ticket), err))
		c.JSON(http.StatusServiceUnavailable, SSOExchangeV2Response{Success: false, Message: "SSO temporarily unavailable"})
		return
	}

	var payload ssoTicketPayloadV2
	if err := common.Unmarshal([]byte(payloadJSON), &payload); err != nil || !validSSOV2Payload(payload, req.Audience) {
		logSSOV2Event("rejected", req.Ticket, req.Audience, payload.UserID)
		c.JSON(http.StatusUnauthorized, SSOExchangeV2Response{Success: false, Message: "invalid or expired ticket"})
		return
	}
	user, err := model.GetUserById(payload.UserID, true)
	if err != nil || user.Status != common.UserStatusEnabled {
		logSSOV2Event("inactive_user", req.Ticket, req.Audience, payload.UserID)
		c.JSON(http.StatusUnauthorized, SSOExchangeV2Response{Success: false, Message: "invalid or expired ticket"})
		return
	}
	logSSOV2Event("consumed", req.Ticket, payload.Audience, payload.UserID)
	c.JSON(http.StatusOK, SSOExchangeV2Response{
		Success:   true,
		UserID:    payload.UserID,
		Audience:  payload.Audience,
		ExpiresAt: time.Unix(payload.ExpiresAt, 0).UTC().Format(time.RFC3339),
	})
}

func validSSOV2Payload(payload ssoTicketPayloadV2, requestedAudience string) bool {
	return payload.UserID > 0 && payload.Audience == ssoStudioAudience && payload.Audience == requestedAudience && payload.Nonce != "" && payload.ExpiresAt > time.Now().Unix()
}

func validateSSOV2RedirectURI(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("redirect_uri is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() == false || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("invalid redirect_uri")
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := host == "studio.geiliapi.com" || host == "localhost"
	if extra := strings.TrimSpace(os.Getenv("SSO_ALLOWED_HOSTS")); extra != "" {
		for _, item := range strings.Split(extra, ",") {
			if strings.EqualFold(strings.TrimSpace(item), host) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return nil, errors.New("redirect host is not allowlisted")
	}
	if host != "localhost" && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("remote redirect must use https")
	}
	if host == "localhost" && !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("localhost redirect scheme is invalid")
	}
	return parsed, nil
}

func ssoTicketHash(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(digest[:6])
}

func logSSOV2Event(event, ticket, audience string, userID int) {
	common.SysLog(fmt.Sprintf("sso_v2 event=%s ticket_hash=%s audience=%s user_id=%d", event, ssoTicketHash(ticket), audience, userID))
}
