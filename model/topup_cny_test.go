package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestTopUpCompletionPathsCreditCNY100KUnits(t *testing.T) {
	testCases := []struct {
		name      string
		userID    int
		tradeNo   string
		provider  string
		amountCNY int64
		moneyCNY  float64
		complete  func(string) error
	}{
		{
			name:      "stripe",
			userID:    7101,
			tradeNo:   "stripe-cny-100k",
			provider:  PaymentProviderStripe,
			amountCNY: 10,
			moneyCNY:  80,
			complete: func(tradeNo string) error {
				return Recharge(tradeNo, "cus_cny_100k", "127.0.0.1")
			},
		},
		{
			name:      "manual epay",
			userID:    7102,
			tradeNo:   "manual-cny-100k",
			provider:  PaymentProviderEpay,
			amountCNY: 3,
			moneyCNY:  3,
			complete: func(tradeNo string) error {
				return ManualCompleteTopUp(tradeNo, "127.0.0.1")
			},
		},
		{
			name:      "waffo",
			userID:    7103,
			tradeNo:   "waffo-cny-100k",
			provider:  PaymentProviderWaffo,
			amountCNY: 5,
			moneyCNY:  5,
			complete: func(tradeNo string) error {
				return RechargeWaffo(tradeNo, "127.0.0.1")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			insertUserForPaymentGuardTest(t, tc.userID, 123)
			require.NoError(t, (&TopUp{
				UserId:          tc.userID,
				Amount:          tc.amountCNY,
				Money:           tc.moneyCNY,
				TradeNo:         tc.tradeNo,
				PaymentMethod:   tc.provider,
				PaymentProvider: tc.provider,
				CreateTime:      time.Now().Unix(),
				Status:          common.TopUpStatusPending,
			}).Insert())

			require.NoError(t, tc.complete(tc.tradeNo))

			require.Equal(t, 123+common.CNYToQuota(float64(tc.amountCNY)), getUserQuotaForPaymentGuardTest(t, tc.userID))
			topUp := GetTopUpByTradeNo(tc.tradeNo)
			require.NotNil(t, topUp)
			require.Equal(t, common.TopUpStatusSuccess, topUp.Status)

			var log Log
			require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", tc.userID, LogTypeTopup).First(&log).Error)
			require.Contains(t, log.Content, "¥")
			require.NotContains(t, strings.ToLower(log.Content), "usd")
			require.NotContains(t, strings.ToLower(log.Content), "quota")
		})
	}
}
