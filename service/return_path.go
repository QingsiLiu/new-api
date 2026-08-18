package service

import "github.com/QuantumNous/new-api/common"

func PaymentReturnURL(suffix string) string {
	base := common.PaymentReturnBaseURL()
	return base + common.ThemeAwarePath(suffix)
}
