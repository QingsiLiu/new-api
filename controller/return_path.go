package controller

import "github.com/QuantumNous/new-api/common"

func paymentReturnPath(suffix string) string {
	base := common.PaymentReturnBaseURL()
	return base + common.ThemeAwarePath(suffix)
}
