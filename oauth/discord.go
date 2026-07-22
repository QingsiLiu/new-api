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
	Register("discord", &DiscordProvider{})
}

// DiscordProvider implements OAuth for Discord
type DiscordProvider struct{}

type discordOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type discordUser struct {
	UID  string `json:"id"`
	ID   string `json:"username"`
	Name string `json:"global_name"`
}

func (p *DiscordProvider) GetName() string {
	return "Discord"
}

func (p *DiscordProvider) IsEnabled() bool {
	return system_setting.GetDiscordSettings().Enabled
}

func (p *DiscordProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "token_exchange_started", oauthSecurityFields{AuthorizationCode: code})

	settings := system_setting.GetDiscordSettings()
	redirectUri := fmt.Sprintf("%s/oauth/discord", system_setting.ServerAddress)
	values := url.Values{}
	values.Set("client_id", settings.ClientId)
	values.Set("client_secret", settings.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "token_request_prepared", oauthSecurityFields{RedirectURI: redirectUri})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://discord.com/api/v10/oauth2/token", strings.NewReader(values.Encode()))
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
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "token_exchange_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "token_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	var discordResponse discordOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&discordResponse)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "token_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if discordResponse.AccessToken == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "access_token_missing", oauthSecurityFields{StatusCode: res.StatusCode})
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "Discord"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "token_exchange_succeeded", oauthSecurityFields{Scope: discordResponse.Scope})

	return &OAuthToken{
		AccessToken:  discordResponse.AccessToken,
		TokenType:    discordResponse.TokenType,
		RefreshToken: discordResponse.RefreshToken,
		ExpiresIn:    discordResponse.ExpiresIn,
		Scope:        discordResponse.Scope,
		IDToken:      discordResponse.IDToken,
	}, nil
}

func (p *DiscordProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "userinfo_request_started", oauthSecurityFields{})

	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "userinfo_request_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "Discord"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "userinfo_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	if res.StatusCode != http.StatusOK {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "userinfo_response_rejected", oauthSecurityFields{StatusCode: res.StatusCode})
		return nil, NewOAuthError(i18n.MsgOAuthGetUserErr, nil)
	}

	var discordUser discordUser
	err = json.NewDecoder(res.Body).Decode(&discordUser)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "userinfo_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if discordUser.UID == "" || discordUser.ID == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "discord", "userinfo_identity_missing", oauthSecurityFields{})
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "Discord"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "discord", "userinfo_succeeded", oauthSecurityFields{
		Subject: discordUser.UID, Username: discordUser.ID,
	})

	return &OAuthUser{
		ProviderUserID: discordUser.UID,
		Username:       discordUser.ID,
		DisplayName:    discordUser.Name,
	}, nil
}

func (p *DiscordProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsDiscordIdAlreadyTaken(providerUserID)
}

func (p *DiscordProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.DiscordId = providerUserID
	return user.FillUserByDiscordId()
}

func (p *DiscordProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.DiscordId = providerUserID
}

func (p *DiscordProvider) GetProviderPrefix() string {
	return "discord_"
}
