package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type CreditPackage struct {
	PackageId           string   `json:"package_id"`
	Credits             int64    `json:"-"`
	CreditsDisplay      string   `json:"credits"`
	BaseCredits         int64    `json:"-"`
	BaseCreditsDisplay  string   `json:"base_credits"`
	BonusCredits        int64    `json:"-"`
	BonusCreditsDisplay string   `json:"bonus_credits"`
	Quota               int      `json:"quota"`
	Currency            string   `json:"currency"`
	PriceMinor          int64    `json:"price_minor"`
	PaymentMethods      []string `json:"payment_methods"`
}

var creditPackages = []CreditPackage{
	newCreditPackage("credits-usd-1000", 1000, "USD", 500, PaymentMethodWaffoPancake),
	newCreditPackage("credits-usd-5000", 5000, "USD", 2500, PaymentMethodWaffoPancake),
	newCreditPackage("credits-usd-10000", 10000, "USD", 5000, PaymentMethodWaffoPancake),
	newCreditPackage("credits-usd-105000", 105000, "USD", 50000, PaymentMethodWaffoPancake),
	newCreditPackage("credits-usd-275000", 275000, "USD", 125000, PaymentMethodWaffoPancake),
	newCreditPackage("credits-cny-1000", 1000, "CNY", 3600, "alipay"),
	newCreditPackage("credits-cny-5000", 5000, "CNY", 18000, "alipay"),
	newCreditPackage("credits-cny-10000", 10000, "CNY", 36000, "alipay"),
	newCreditPackage("credits-cny-105000", 105000, "CNY", 360000, "alipay"),
	newCreditPackage("credits-cny-275000", 275000, "CNY", 900000, "alipay"),
}

func newCreditPackage(id string, credits int64, currency string, priceMinor int64, paymentMethods ...string) CreditPackage {
	quota, _ := creditsToQuotaForPackage(credits)
	baseCredits, bonusCredits := creditPackageBreakdown(credits)
	return CreditPackage{
		PackageId:           id,
		Credits:             credits,
		CreditsDisplay:      strings.TrimSpace(idCreditsString(credits)),
		BaseCredits:         baseCredits,
		BaseCreditsDisplay:  idCreditsString(baseCredits),
		BonusCredits:        bonusCredits,
		BonusCreditsDisplay: idCreditsString(bonusCredits),
		Quota:               quota,
		Currency:            currency,
		PriceMinor:          priceMinor,
		PaymentMethods:      paymentMethods,
	}
}

func creditPackageBreakdown(total int64) (int64, int64) {
	switch total {
	case 105000:
		return 100000, 5000
	case 275000:
		return 250000, 25000
	default:
		return total, 0
	}
}

func creditsToQuotaForPackage(credits int64) (int, bool) {
	return common.CreditsToQuota(credits)
}

func idCreditsString(credits int64) string {
	if credits == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for credits > 0 {
		i--
		buf[i] = digits[credits%10]
		credits /= 10
	}
	return string(buf[i:])
}

func ListCreditPackages() []CreditPackage {
	result := make([]CreditPackage, len(creditPackages))
	for i := range creditPackages {
		result[i] = creditPackages[i]
		result[i].PaymentMethods = append([]string(nil), creditPackages[i].PaymentMethods...)
	}
	return result
}

func FindCreditPackage(packageID string) (CreditPackage, bool) {
	packageID = strings.TrimSpace(packageID)
	for _, pkg := range creditPackages {
		if pkg.PackageId == packageID {
			return pkg, true
		}
	}
	return CreditPackage{}, false
}

func (p CreditPackage) SupportsPaymentMethod(method string) bool {
	method = strings.TrimSpace(method)
	for _, candidate := range p.PaymentMethods {
		if candidate == method {
			return true
		}
	}
	return false
}
