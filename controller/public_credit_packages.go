package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type publicCreditPackage struct {
	PackageID    string `json:"package_id"`
	Credits      string `json:"credits"`
	BaseCredits  string `json:"base_credits"`
	BonusCredits string `json:"bonus_credits"`
	Quota        int    `json:"quota"`
	Currency     string `json:"currency"`
	PriceMinor   int64  `json:"price_minor"`
}

// GetPublicCreditPackages returns only the immutable customer-facing package
// snapshot. Payment providers, merchant state, and checkout remain private.
func GetPublicCreditPackages(c *gin.Context) {
	enabled := common.CreditsV1Enabled()
	packages := make([]publicCreditPackage, 0)
	if enabled {
		serverPackages := model.ListCreditPackages()
		packages = make([]publicCreditPackage, 0, len(serverPackages))
		for _, pkg := range serverPackages {
			packages = append(packages, publicCreditPackage{
				PackageID:    pkg.PackageId,
				Credits:      pkg.CreditsDisplay,
				BaseCredits:  pkg.BaseCreditsDisplay,
				BonusCredits: pkg.BonusCreditsDisplay,
				Quota:        pkg.Quota,
				Currency:     pkg.Currency,
				PriceMinor:   pkg.PriceMinor,
			})
		}
	}
	c.Header("Cache-Control", "public, max-age=300")
	common.ApiSuccess(c, gin.H{
		"credits_enabled":  enabled,
		"quota_per_credit": common.CreditsQuotaUnit,
		"packages":         packages,
	})
}
