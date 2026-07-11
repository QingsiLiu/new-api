package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func IsQuotaMigrationInProgress() bool {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[common.QuotaMigrationInProgressKey]
	common.OptionMapRWMutex.RUnlock()
	if isTruthyOption(raw) {
		return true
	}
	if DB == nil {
		return false
	}

	var option Option
	if err := DB.Select("value").Where("key = ?", common.QuotaMigrationInProgressKey).First(&option).Error; err != nil {
		return false
	}
	return isTruthyOption(option.Value)
}

func isTruthyOption(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
