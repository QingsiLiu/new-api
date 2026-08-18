package passkey

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/go-webauthn/webauthn/protocol"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
)

const (
	RegistrationSessionKey = "passkey_registration_session"
	LoginSessionKey        = "passkey_login_session"
	VerifySessionKey       = "passkey_verify_session"
)

// BuildWebAuthn constructs a WebAuthn instance using the current passkey settings and request context.
func BuildWebAuthn(r *http.Request) (*webauthn.WebAuthn, error) {
	settings := system_setting.GetPasskeySettings()
	if settings == nil {
		return nil, errors.New("未找到 Passkey 设置")
	}

	displayName := strings.TrimSpace(settings.RPDisplayName)
	if displayName == "" {
		displayName = common.SystemName
	}

	origins, err := resolveOrigins(r, settings)
	if err != nil {
		return nil, err
	}

	rpID, err := resolveRPID(r, settings, origins)
	if err != nil {
		return nil, err
	}

	selection := protocol.AuthenticatorSelection{
		ResidentKey:        protocol.ResidentKeyRequirementRequired,
		RequireResidentKey: protocol.ResidentKeyRequired(),
		UserVerification:   protocol.UserVerificationRequirement(settings.UserVerification),
	}
	if selection.UserVerification == "" {
		selection.UserVerification = protocol.VerificationPreferred
	}
	if attachment := strings.TrimSpace(settings.AttachmentPreference); attachment != "" {
		selection.AuthenticatorAttachment = protocol.AuthenticatorAttachment(attachment)
	}

	config := &webauthn.Config{
		RPID:                   rpID,
		RPDisplayName:          displayName,
		RPOrigins:              origins,
		AuthenticatorSelection: selection,
		Debug:                  common.DebugEnabled,
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    2 * time.Minute,
				TimeoutUVD: 2 * time.Minute,
			},
		},
	}

	return webauthn.New(config)
}

func requestPasskeyProfile(r *http.Request) (string, string) {
	if r == nil {
		return "", ""
	}
	// A direct public request has a recognized Host. Requests from the
	// geili-web bridge reach the engine over its container hostname, so the
	// bridge supplies an exact, allowlisted X-Forwarded-Host instead. Prefer a
	// recognized direct Host and never accept arbitrary forwarded domains.
	for _, candidate := range []string{r.Host, r.Header.Get("X-Forwarded-Host")} {
		if strings.Contains(candidate, ",") {
			continue
		}
		host := strings.ToLower(strings.TrimSpace(candidate))
		if parsed, err := url.Parse("https://" + host); err == nil {
			host = parsed.Hostname()
		}
		switch host {
		case "auapi.ai", "admin.auapi.ai", "next.auapi.ai":
			return "auapi.ai", "https://" + host
		case "geiliapi.com", "admin.geiliapi.com", "next.geiliapi.com":
			return "geiliapi.com", "https://" + host
		}
	}
	return "", ""
}

func RPIDForRequest(r *http.Request) string {
	rpID, _ := requestPasskeyProfile(r)
	return rpID
}

func OriginForRequest(r *http.Request) string {
	_, origin := requestPasskeyProfile(r)
	return origin
}

func resolveOrigins(r *http.Request, settings *system_setting.PasskeySettings) ([]string, error) {
	if _, origin := requestPasskeyProfile(r); origin != "" {
		return []string{origin}, nil
	}
	originsStr := strings.TrimSpace(settings.Origins)
	if originsStr != "" {
		originList := strings.Split(originsStr, ",")
		origins := make([]string, 0, len(originList))
		for _, origin := range originList {
			trimmed := strings.TrimSpace(origin)
			if trimmed == "" {
				continue
			}
			if !settings.AllowInsecureOrigin && strings.HasPrefix(strings.ToLower(trimmed), "http://") {
				return nil, fmt.Errorf("Passkey 不允许使用不安全的 Origin: %s", trimmed)
			}
			origins = append(origins, trimmed)
		}
		if len(origins) == 0 {
			// 如果配置了Origins但过滤后为空，使用自动推导
			goto autoDetect
		}
		return origins, nil
	}

autoDetect:
	scheme := detectScheme(r)
	if scheme == "http" && !settings.AllowInsecureOrigin && r.Host != "localhost" && r.Host != "127.0.0.1" && !strings.HasPrefix(r.Host, "127.0.0.1:") && !strings.HasPrefix(r.Host, "localhost:") {
		return nil, fmt.Errorf("Passkey 仅支持 HTTPS，当前访问: %s://%s，请在 Passkey 设置中允许不安全 Origin 或配置 HTTPS", scheme, r.Host)
	}
	// 优先使用请求的完整Host（包含端口）
	host := r.Host

	// 如果无法从请求获取Host，尝试从ServerAddress获取
	if host == "" && system_setting.ServerAddress != "" {
		if parsed, err := url.Parse(system_setting.ServerAddress); err == nil && parsed.Host != "" {
			host = parsed.Host
			if scheme == "" && parsed.Scheme != "" {
				scheme = parsed.Scheme
			}
		}
	}
	if host == "" {
		return nil, fmt.Errorf("无法确定 Passkey 的 Origin，请在系统设置或 Passkey 设置中指定。当前 Host: '%s', ServerAddress: '%s'", r.Host, system_setting.ServerAddress)
	}
	if scheme == "" {
		scheme = "https"
	}
	origin := fmt.Sprintf("%s://%s", scheme, host)
	return []string{origin}, nil
}

func resolveRPID(r *http.Request, settings *system_setting.PasskeySettings, origins []string) (string, error) {
	if rpID, _ := requestPasskeyProfile(r); rpID != "" {
		return rpID, nil
	}
	rpID := strings.TrimSpace(settings.RPID)
	if rpID != "" {
		return hostWithoutPort(rpID), nil
	}
	if len(origins) == 0 {
		return "", errors.New("Passkey 未配置 Origin，无法推导 RPID")
	}
	parsed, err := url.Parse(origins[0])
	if err != nil {
		return "", fmt.Errorf("无法解析 Passkey Origin: %w", err)
	}
	return hostWithoutPort(parsed.Host), nil
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if host, _, err := net.SplitHostPort(host); err == nil {
			return host
		}
	}
	return host
}

func detectScheme(r *http.Request) string {
	if r == nil {
		return ""
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		parts := strings.Split(proto, ",")
		return strings.ToLower(strings.TrimSpace(parts[0]))
	}
	if r.TLS != nil {
		return "https"
	}
	if r.URL != nil && r.URL.Scheme != "" {
		return strings.ToLower(r.URL.Scheme)
	}
	if r.Header.Get("X-Forwarded-Protocol") != "" {
		return strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Protocol")))
	}
	return "http"
}
