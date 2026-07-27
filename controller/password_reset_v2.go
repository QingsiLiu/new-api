package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	passwordResetV2EnabledEnv = "GEILI_PASSWORD_RESET_V2"
	passwordResetV2Purpose    = "password_reset_v2"
	passwordResetV2KeyPrefix  = "password_reset_v2:"
	passwordResetTicketBytes  = 32
)

var (
	errPasswordResetTicketInvalid     = errors.New("invalid or expired password reset ticket")
	errPasswordResetTicketUnavailable = errors.New("password reset ticket store unavailable")
	passwordResetMemoryTickets        = struct {
		sync.Mutex
		items map[string]passwordResetTicketPayload
	}{items: make(map[string]passwordResetTicketPayload)}
)

type passwordResetTicketPayload struct {
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expires_at"`
	Purpose   string `json:"purpose"`
}

type PasswordResetV2Request struct {
	Ticket string `json:"ticket"`
}

func passwordResetV2Enabled() bool {
	return common.GetEnvOrDefaultBool(passwordResetV2EnabledEnv, false)
}

func issuePasswordResetTicket(email string) (string, error) {
	email = model.NormalizeEmail(email)
	if email == "" {
		return "", errPasswordResetTicketInvalid
	}
	raw := make([]byte, passwordResetTicketBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	payload := passwordResetTicketPayload{
		Email:     email,
		ExpiresAt: time.Now().UTC().Add(passwordResetTicketTTL()).Unix(),
		Purpose:   passwordResetV2Purpose,
	}
	if err := storePasswordResetTicket(passwordResetTicketHash(ticket), payload); err != nil {
		return "", err
	}
	return ticket, nil
}

func consumePasswordResetTicket(ticket string) (string, error) {
	ticket = strings.TrimSpace(ticket)
	raw, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil || len(raw) != passwordResetTicketBytes {
		return "", errPasswordResetTicketInvalid
	}
	payload, err := consumeStoredPasswordResetTicket(passwordResetTicketHash(ticket))
	if err != nil {
		return "", err
	}
	if payload.Purpose != passwordResetV2Purpose || payload.ExpiresAt <= time.Now().Unix() || model.NormalizeEmail(payload.Email) != payload.Email || payload.Email == "" {
		return "", errPasswordResetTicketInvalid
	}
	return payload.Email, nil
}

func passwordResetTicketTTL() time.Duration {
	return time.Duration(common.VerificationValidMinutes) * time.Minute
}

func passwordResetTicketHash(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(digest[:])
}

func storePasswordResetTicket(hash string, payload passwordResetTicketPayload) error {
	if common.RedisEnabled {
		if common.RDB == nil {
			return errPasswordResetTicketUnavailable
		}
		encoded, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		if err := common.RedisSet(passwordResetV2KeyPrefix+hash, string(encoded), passwordResetTicketTTL()); err != nil {
			return fmt.Errorf("store password reset ticket: %w", err)
		}
		return nil
	}

	passwordResetMemoryTickets.Lock()
	defer passwordResetMemoryTickets.Unlock()
	purgeExpiredPasswordResetMemoryTickets(time.Now().Unix())
	passwordResetMemoryTickets.items[hash] = payload
	return nil
}

func consumeStoredPasswordResetTicket(hash string) (passwordResetTicketPayload, error) {
	if common.RedisEnabled {
		if common.RDB == nil {
			return passwordResetTicketPayload{}, errPasswordResetTicketUnavailable
		}
		encoded, err := common.RedisGetDel(passwordResetV2KeyPrefix + hash)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return passwordResetTicketPayload{}, errPasswordResetTicketInvalid
			}
			return passwordResetTicketPayload{}, errPasswordResetTicketUnavailable
		}
		var payload passwordResetTicketPayload
		if err := common.Unmarshal([]byte(encoded), &payload); err != nil {
			return passwordResetTicketPayload{}, errPasswordResetTicketInvalid
		}
		return payload, nil
	}

	passwordResetMemoryTickets.Lock()
	defer passwordResetMemoryTickets.Unlock()
	payload, ok := passwordResetMemoryTickets.items[hash]
	delete(passwordResetMemoryTickets.items, hash)
	if !ok {
		return passwordResetTicketPayload{}, errPasswordResetTicketInvalid
	}
	return payload, nil
}

func purgeExpiredPasswordResetMemoryTickets(now int64) {
	for hash, payload := range passwordResetMemoryTickets.items {
		if payload.ExpiresAt <= now {
			delete(passwordResetMemoryTickets.items, hash)
		}
	}
}

func ResetPasswordV2(c *gin.Context) {
	setPasswordResetNoStore(c)
	if !passwordResetV2Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "password reset v2 is not enabled"})
		return
	}
	var req PasswordResetV2Request
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordResetLinkInvalid)
		return
	}
	email, err := consumePasswordResetTicket(req.Ticket)
	if err != nil {
		if errors.Is(err, errPasswordResetTicketUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "password reset temporarily unavailable"})
			return
		}
		common.ApiErrorI18n(c, i18n.MsgUserPasswordResetLinkInvalid)
		return
	}
	password := common.GenerateVerificationCode(12)
	if err := model.ResetUserPasswordByEmail(email, password); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordResetLinkInvalid)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
}

func setPasswordResetNoStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
}
