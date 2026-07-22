package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("linuxdo", &LinuxDOProvider{})
}

// LinuxDOProvider implements OAuth for Linux DO
type LinuxDOProvider struct{}

type linuxdoUser struct {
	Id         int    `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	TrustLevel int    `json:"trust_level"`
	Silenced   bool   `json:"silenced"`
}

func (p *LinuxDOProvider) GetName() string {
	return "Linux DO"
}

func (p *LinuxDOProvider) IsEnabled() bool {
	return common.LinuxDOOAuthEnabled
}

func (p *LinuxDOProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "token_exchange_started", oauthSecurityFields{AuthorizationCode: code})

	// Get access token using Basic auth
	tokenEndpoint := common.GetEnvOrDefaultString("LINUX_DO_TOKEN_ENDPOINT", "https://connect.linux.do/oauth2/token")
	credentials := common.LinuxDOClientId + ":" + common.LinuxDOClientSecret
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))

	// Get redirect URI from request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/api/oauth/linuxdo", scheme, c.Request.Host)

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "token_request_prepared", oauthSecurityFields{
		Endpoint: tokenEndpoint, RedirectURI: redirectURI,
	})

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "token_exchange_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Linux DO"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "token_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenRes); err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "token_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if tokenRes.AccessToken == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "access_token_missing", oauthSecurityFields{StatusCode: res.StatusCode, Reason: "provider_error"})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Linux DO"}, tokenRes.Message)
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "token_exchange_succeeded", oauthSecurityFields{})

	return &OAuthToken{
		AccessToken: tokenRes.AccessToken,
	}, nil
}

func (p *LinuxDOProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	userEndpoint := common.GetEnvOrDefaultString("LINUX_DO_USER_ENDPOINT", "https://connect.linux.do/api/user")

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "userinfo_request_started", oauthSecurityFields{Endpoint: userEndpoint})

	req, err := http.NewRequestWithContext(ctx, "GET", userEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "userinfo_request_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Linux DO"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "userinfo_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	var linuxdoUser linuxdoUser
	if err := json.NewDecoder(res.Body).Decode(&linuxdoUser); err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "userinfo_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if linuxdoUser.Id == 0 {
		logOAuthSecurityEvent(ctx, oauthLogError, "linuxdo", "userinfo_identity_missing", oauthSecurityFields{})
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "Linux DO"})
	}

	active := linuxdoUser.Active
	silenced := linuxdoUser.Silenced
	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "userinfo_received", oauthSecurityFields{
		Subject: strconv.Itoa(linuxdoUser.Id), Username: linuxdoUser.Username, RequiredTrust: common.LinuxDOMinimumTrustLevel,
		CurrentTrust: linuxdoUser.TrustLevel, Active: &active, Silenced: &silenced,
	})

	// Check trust level
	if linuxdoUser.TrustLevel < common.LinuxDOMinimumTrustLevel {
		logOAuthSecurityEvent(ctx, oauthLogWarn, "linuxdo", "trust_level_rejected", oauthSecurityFields{
			Subject: strconv.Itoa(linuxdoUser.Id), RequiredTrust: common.LinuxDOMinimumTrustLevel, CurrentTrust: linuxdoUser.TrustLevel,
		})
		return nil, &TrustLevelError{
			Required: common.LinuxDOMinimumTrustLevel,
			Current:  linuxdoUser.TrustLevel,
		}
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "linuxdo", "userinfo_succeeded", oauthSecurityFields{
		Subject: strconv.Itoa(linuxdoUser.Id), Username: linuxdoUser.Username,
	})

	return &OAuthUser{
		ProviderUserID: strconv.Itoa(linuxdoUser.Id),
		Username:       linuxdoUser.Username,
		DisplayName:    linuxdoUser.Name,
		Extra: map[string]any{
			"trust_level": linuxdoUser.TrustLevel,
			"active":      linuxdoUser.Active,
			"silenced":    linuxdoUser.Silenced,
		},
	}, nil
}

func (p *LinuxDOProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsLinuxDOIdAlreadyTaken(providerUserID)
}

func (p *LinuxDOProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.LinuxDOId = providerUserID
	return user.FillUserByLinuxDOId()
}

func (p *LinuxDOProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.LinuxDOId = providerUserID
}

func (p *LinuxDOProvider) GetProviderPrefix() string {
	return "linuxdo_"
}

// TrustLevelError indicates the user's trust level is too low
type TrustLevelError struct {
	Required int
	Current  int
}

func (e *TrustLevelError) Error() string {
	return "trust level too low"
}
