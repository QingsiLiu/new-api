package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func GetCallbackAddress() string {
	if operation_setting.CustomCallbackAddress == "" {
		return common.PaymentCallbackBaseURL()
	}
	return operation_setting.CustomCallbackAddress
}
