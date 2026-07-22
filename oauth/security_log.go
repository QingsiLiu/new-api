package oauth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

type oauthLogLevel uint8

const (
	oauthLogDebug oauthLogLevel = iota
	oauthLogInfo
	oauthLogWarn
	oauthLogError
)

type oauthSecurityFields struct {
	AuthorizationCode string
	ResponseBody      []byte
	Subject           string
	Username          string
	Email             string
	Endpoint          string
	RedirectURI       string
	Scope             string
	StatusCode        int
	AuthStyle         int
	RequiredTrust     int
	CurrentTrust      int
	Active            *bool
	Silenced          *bool
	Field             string
	Operator          string
	Reason            string
	Err               error
}

func logOAuthSecurityEvent(ctx context.Context, level oauthLogLevel, provider, event string, fields oauthSecurityFields) {
	record := map[string]any{
		"component": "authentication",
		"protocol":  "oauth",
		"provider":  oauthLogToken(provider),
		"event":     oauthLogToken(event),
	}
	record["authorization_code_present"] = strings.TrimSpace(fields.AuthorizationCode) != ""
	if fields.AuthorizationCode != "" {
		record["authorization_code_sha256"] = oauthLogDigest(fields.AuthorizationCode)
	}
	if fields.ResponseBody != nil {
		record["response_bytes"] = len(fields.ResponseBody)
		record["response_sha256"] = oauthLogDigest(string(fields.ResponseBody))
	}
	if fields.Subject != "" {
		record["subject_sha256"] = oauthLogDigest(fields.Subject)
	}
	if fields.Username != "" {
		record["username_sha256"] = oauthLogDigest(fields.Username)
	}
	if fields.Email != "" {
		record["email_sha256"] = oauthLogDigest(fields.Email)
	}
	if endpoint := oauthLogEndpoint(fields.Endpoint); endpoint != "" {
		record["endpoint"] = endpoint
	}
	if redirectURI := oauthLogEndpoint(fields.RedirectURI); redirectURI != "" {
		record["redirect_uri"] = redirectURI
	}
	if fields.Scope != "" {
		record["scope_count"] = len(strings.Fields(fields.Scope))
	}
	if fields.StatusCode > 0 {
		record["status_code"] = fields.StatusCode
	}
	if fields.AuthStyle > 0 {
		record["auth_style"] = fields.AuthStyle
	}
	if fields.RequiredTrust > 0 {
		record["required_trust_level"] = fields.RequiredTrust
	}
	if fields.CurrentTrust > 0 {
		record["current_trust_level"] = fields.CurrentTrust
	}
	if fields.Active != nil {
		record["active"] = *fields.Active
	}
	if fields.Silenced != nil {
		record["silenced"] = *fields.Silenced
	}
	if fields.Field != "" {
		record["field"] = oauthLogToken(fields.Field)
	}
	if fields.Operator != "" {
		record["operator"] = oauthLogToken(fields.Operator)
	}
	if fields.Reason != "" {
		record["reason"] = oauthLogToken(fields.Reason)
	}
	if fields.Err != nil {
		record["error_type"] = reflect.TypeOf(fields.Err).String()
		record["error_sha256"] = oauthLogDigest(fields.Err.Error())
	}

	encoded, err := common.Marshal(record)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("oauth security log marshal failed: %T", err))
		return
	}
	message := string(encoded)
	switch level {
	case oauthLogInfo:
		logger.LogInfo(ctx, message)
	case oauthLogWarn:
		logger.LogWarn(ctx, message)
	case oauthLogError:
		logger.LogError(ctx, message)
	default:
		logger.LogDebug(ctx, message)
	}
}

func oauthLogDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:8])
}

func oauthLogEndpoint(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath()
}

func oauthLogToken(value string) string {
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
