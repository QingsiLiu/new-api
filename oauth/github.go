package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("github", &GitHubProvider{})
}

// GitHubProvider implements OAuth for GitHub
type GitHubProvider struct{}

type gitHubOAuthResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

type gitHubUser struct {
	Id    int64  `json:"id"`    // GitHub numeric ID (permanent, never changes)
	Login string `json:"login"` // GitHub username (can be changed by user)
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (p *GitHubProvider) GetName() string {
	return "GitHub"
}

func (p *GitHubProvider) IsEnabled() bool {
	return common.GitHubOAuthEnabled
}

func (p *GitHubProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "token_exchange_started", oauthSecurityFields{AuthorizationCode: code})

	values := map[string]string{
		"client_id":     common.GitHubClientId,
		"client_secret": common.GitHubClientSecret,
		"code":          code,
	}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "token_exchange_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "token_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	var oAuthResponse gitHubOAuthResponse
	err = json.NewDecoder(res.Body).Decode(&oAuthResponse)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "token_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if oAuthResponse.AccessToken == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "access_token_missing", oauthSecurityFields{StatusCode: res.StatusCode})
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": "GitHub"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "token_exchange_succeeded", oauthSecurityFields{Scope: oAuthResponse.Scope})

	return &OAuthToken{
		AccessToken: oAuthResponse.AccessToken,
		TokenType:   oAuthResponse.TokenType,
		Scope:       oAuthResponse.Scope,
	}, nil
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "userinfo_request_started", oauthSecurityFields{})

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := http.Client{
		Timeout: 20 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "userinfo_request_failed", oauthSecurityFields{Err: err})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": "GitHub"}, err.Error())
	}
	defer res.Body.Close()

	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "userinfo_response_received", oauthSecurityFields{StatusCode: res.StatusCode})

	// Check for non-200 status codes before attempting to decode
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "userinfo_response_rejected", oauthSecurityFields{
			StatusCode: res.StatusCode, ResponseBody: body,
		})
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": "GitHub"}, fmt.Sprintf("status %d", res.StatusCode))
	}

	var githubUser gitHubUser
	err = json.NewDecoder(res.Body).Decode(&githubUser)
	if err != nil {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "userinfo_response_invalid", oauthSecurityFields{StatusCode: res.StatusCode, Err: err})
		return nil, err
	}

	if githubUser.Id == 0 || githubUser.Login == "" {
		logOAuthSecurityEvent(ctx, oauthLogError, "github", "userinfo_identity_missing", oauthSecurityFields{})
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": "GitHub"})
	}

	logOAuthSecurityEvent(ctx, oauthLogDebug, "github", "userinfo_succeeded", oauthSecurityFields{
		Subject: strconv.FormatInt(githubUser.Id, 10), Username: githubUser.Login, Email: githubUser.Email,
	})

	return &OAuthUser{
		ProviderUserID: strconv.FormatInt(githubUser.Id, 10), // Use numeric ID as primary identifier
		Username:       githubUser.Login,
		DisplayName:    githubUser.Name,
		Email:          githubUser.Email,
		Extra: map[string]any{
			"legacy_id": githubUser.Login, // Store login for migration from old accounts
		},
	}, nil
}

func (p *GitHubProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsGitHubIdAlreadyTaken(providerUserID)
}

func (p *GitHubProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.GitHubId = providerUserID
	return user.FillUserByGitHubId()
}

func (p *GitHubProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (p *GitHubProvider) GetProviderPrefix() string {
	return "github_"
}
