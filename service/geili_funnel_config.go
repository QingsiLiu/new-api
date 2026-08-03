package service

import (
	"errors"
	"os"

	"github.com/QuantumNous/new-api/common"
)

var ErrGeiliFunnelSecretInvalid = errors.New("geili funnel configuration invalid")

type GeiliFunnelConfig struct {
	Enabled         bool
	Secret          string
	UnauthorizedRPM int
	WriteRPM        int
	ReadRPM         int
}

func LoadGeiliFunnelConfig() (GeiliFunnelConfig, error) {
	config := GeiliFunnelConfig{
		Enabled:         common.GetEnvOrDefaultBool("GEILI_FUNNEL_ENABLED", false),
		Secret:          os.Getenv("GEILI_FUNNEL_SECRET"),
		UnauthorizedRPM: positiveFunnelEnv("GEILI_FUNNEL_UNAUTHORIZED_RPM", 60),
		WriteRPM:        positiveFunnelEnv("GEILI_FUNNEL_WRITE_RPM", 1200),
		ReadRPM:         positiveFunnelEnv("GEILI_FUNNEL_READ_RPM", 60),
	}
	if config.Enabled && len([]byte(config.Secret)) < 32 {
		return GeiliFunnelConfig{}, ErrGeiliFunnelSecretInvalid
	}
	return config, nil
}

func positiveFunnelEnv(name string, fallback int) int {
	value := common.GetEnvOrDefault(name, fallback)
	if value <= 0 {
		return fallback
	}
	return value
}
