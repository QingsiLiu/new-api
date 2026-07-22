package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

type authLogLevel uint8

const (
	authLogInfo authLogLevel = iota
	authLogWarn
	authLogError
)

type authSecurityFields struct {
	Provider   string
	Method     string
	Username   string
	Email      string
	Subject    string
	Credential string
	UserID     int
	Reason     string
	Err        error
}

func logAuthSecurityEvent(ctx context.Context, level authLogLevel, event string, fields authSecurityFields) {
	record := map[string]any{
		"component": "authentication",
		"event":     authLogToken(event),
	}
	if fields.Provider != "" {
		record["provider"] = authLogToken(fields.Provider)
	}
	if fields.Method != "" {
		record["method"] = authLogToken(fields.Method)
	}
	if fields.Username != "" {
		record["username_sha256"] = authLogDigest(fields.Username)
	}
	if fields.Email != "" {
		record["email_sha256"] = authLogDigest(fields.Email)
	}
	if fields.Subject != "" {
		record["subject_sha256"] = authLogDigest(fields.Subject)
	}
	if fields.Credential != "" {
		record["credential_present"] = true
		record["credential_bytes"] = len(fields.Credential)
		record["credential_sha256"] = authLogDigest(fields.Credential)
	}
	if fields.UserID > 0 {
		record["user_id"] = fields.UserID
	}
	if fields.Reason != "" {
		record["reason"] = authLogToken(fields.Reason)
	}
	if fields.Err != nil {
		record["error_type"] = reflect.TypeOf(fields.Err).String()
		record["error_sha256"] = authLogDigest(fields.Err.Error())
	}

	encoded, err := common.Marshal(record)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("authentication security log marshal failed: %T", err))
		return
	}
	message := string(encoded)
	switch level {
	case authLogWarn:
		logger.LogWarn(ctx, message)
	case authLogError:
		logger.LogError(ctx, message)
	default:
		logger.LogInfo(ctx, message)
	}
}

func authLogDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:8])
}

func authLogToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 96 {
		return "unknown"
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:-/", r) {
			continue
		}
		return "unknown"
	}
	return value
}
