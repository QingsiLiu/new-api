package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("oidc", &OIDCProvider{})
}

// OIDCProvider implements OAuth for OIDC
type OIDCProvider struct{}

type oidcOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type oidcUser struct {
	OpenID            string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Picture           string `json:"picture"`
}

func (p *OIDCProvider) GetName() string {
	return "OIDC"
}

func (p *OIDCProvider) IsEnabled() bool {
	return system_setting.GetOIDCSettings().Enabled
}

func (p *OIDCProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "token_exchange_started", oauthSecurityFields{AuthorizationCode: code})

	settings := system_setting.GetOIDCSettings()
	// 门面(登录页)域名与 ServerAddress 分离部署时，换 token 的 redirect_uri 必须与
	// 授权跳转、IdP 后台登记值完全一致（Google 等严格校验），否则 invalid_grant。
	// oidc.redirect_uri 为空则回落 ServerAddress 拼接（上游原行为）。
	redirectUri := strings.TrimSpace(settings.RedirectUri)
	if redirectUri == "" {
		redirectUri = fmt.Sprintf("%s/oauth/oidc", system_setting.ServerAddress)
	}
	values := url.Values{}
	values.Set("client_id", settings.ClientId)
	values.Set("client_secret", settings.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "token_request_prepared", oauthSecurityFields{
		Endpoint: settings.TokenEndpoint, RedirectURI: redirectUri,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", settings.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "token_exchange_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "OIDC"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "token_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	var oidcResponse oidcOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&oidcResponse)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "token_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if oidcResponse.AccessToken == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "access_token_missing", oauthSecurityFields{StatusCode: res.StatusCode})
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "OIDC"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "token_exchange_succeeded", oauthSecurityFields{Scope: oidcResponse.Scope})

	return &OAuthToken{
		AccessToken:  oidcResponse.AccessToken,
		TokenType:    oidcResponse.TokenType,
		RefreshToken: oidcResponse.RefreshToken,
		ExpiresIn:    oidcResponse.ExpiresIn,
		Scope:        oidcResponse.Scope,
		IDToken:      oidcResponse.IDToken,
	}, nil
}

func (p *OIDCProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	settings := system_setting.GetOIDCSettings()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "userinfo_request_started", oauthSecurityFields{Endpoint: settings.UserInfoEndpoint})

	req, err := http.NewRequestWithContext(ctx, "GET", settings.UserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "userinfo_request_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "OIDC"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "userinfo_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	if res.StatusCode != http.StatusOK {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "userinfo_response_rejected", oauthSecurityFields{StatusCode: res.StatusCode})
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}

	var oidcUser oidcUser
	err = json.NewDecoder(res.Body).Decode(&oidcUser)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "userinfo_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if oidcUser.OpenID == "" || oidcUser.Email == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "oidc", "userinfo_identity_missing", oauthSecurityFields{
			Subject: oidcUser.OpenID, Email: oidcUser.Email,
		})
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "OIDC"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "oidc", "userinfo_succeeded", oauthSecurityFields{
		Subject: oidcUser.OpenID, Username: oidcUser.PreferredUsername, Email: oidcUser.Email,
	})

	return &OAuthUser{
		ProviderUserID: oidcUser.OpenID,
		Username:       oidcUser.PreferredUsername,
		DisplayName:    oidcUser.Name,
		Email:          oidcUser.Email,
	}, nil
}

func (p *OIDCProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsOidcIdAlreadyTaken(providerUserID)
}

func (p *OIDCProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.OidcId = providerUserID
	return user.FillUserByOidcId()
}

func (p *OIDCProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.OidcId = providerUserID
}

func (p *OIDCProvider) GetProviderPrefix() string {
	return "oidc_"
}
