package helper

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

func quotaPerMillionFromCentiCredits(centiCredits int64) int64 {
	return centiCredits * int64(common.CreditsQuotaUnit) / 100
}

func creditsV1TextPricing(modelName string) (*types.CreditsTextPricing, bool) {
	return model.ResolveLegacyTextPricingSnapshot(modelName)
}

func CreditsV1TextPricingForPublicCatalog(modelName string) (*types.CreditsTextPricing, bool) {
	pricing, ok, err := model.ResolveEffectiveTextPricing(modelName)
	if err != nil {
		return nil, false
	}
	return pricing, ok
}
