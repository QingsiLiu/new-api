package model

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCreditPackagesAreFixedServerSnapshots(t *testing.T) {
	packages := ListCreditPackages()
	require.Len(t, packages, 10)

	usd, ok := FindCreditPackage("credits-usd-105000")
	require.True(t, ok)
	require.EqualValues(t, 105000, usd.Credits)
	require.EqualValues(t, 100000, usd.BaseCredits)
	require.EqualValues(t, 5000, usd.BonusCredits)
	require.Equal(t, 378000000, usd.Quota)
	require.Equal(t, "USD", usd.Currency)
	require.EqualValues(t, 50000, usd.PriceMinor)
	require.True(t, usd.SupportsPaymentMethod(PaymentMethodWaffoPancake))
	require.False(t, usd.SupportsPaymentMethod("alipay"))

	cny, ok := FindCreditPackage("credits-cny-275000")
	require.True(t, ok)
	require.Equal(t, 990000000, cny.Quota)
	require.EqualValues(t, 250000, cny.BaseCredits)
	require.EqualValues(t, 25000, cny.BonusCredits)
	require.EqualValues(t, 850000, cny.PriceMinor)
	require.True(t, cny.SupportsPaymentMethod("alipay"))

	_, ok = FindCreditPackage("credits-usd-1000-tampered")
	require.False(t, ok)
}

func TestCNYCreditPackagesUseFixedCNYPerUSDExchangeRate(t *testing.T) {
	testCases := []struct {
		packageID     string
		usdPriceMinor int64
		cnyPriceMinor int64
	}{
		{packageID: "credits-cny-1000", usdPriceMinor: 500, cnyPriceMinor: 3400},
		{packageID: "credits-cny-5000", usdPriceMinor: 2500, cnyPriceMinor: 17000},
		{packageID: "credits-cny-10000", usdPriceMinor: 5000, cnyPriceMinor: 34000},
		{packageID: "credits-cny-105000", usdPriceMinor: 50000, cnyPriceMinor: 340000},
		{packageID: "credits-cny-275000", usdPriceMinor: 125000, cnyPriceMinor: 850000},
	}

	for _, testCase := range testCases {
		t.Run(testCase.packageID, func(t *testing.T) {
			pkg, ok := FindCreditPackage(testCase.packageID)
			require.True(t, ok)
			require.EqualValues(t, testCase.cnyPriceMinor, pkg.PriceMinor)
			require.EqualValues(t, testCase.usdPriceMinor*common.CNYPerUSDCents/100, pkg.PriceMinor)
		})
	}
}

func TestSignupCreditGrantIsIdempotentAndIdentityBound(t *testing.T) {
	truncateTables(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	first := &User{Id: 8201, Username: "credits-signup-1", Status: common.UserStatusEnabled, AffCode: "cs01"}
	second := &User{Id: 8202, Username: "credits-signup-2", Status: common.UserStatusEnabled, AffCode: "cs02"}
	require.NoError(t, DB.Create(first).Error)
	require.NoError(t, DB.Create(second).Error)

	require.NoError(t, GrantSignupCredits(first.Id, "email_verified", "email:verified@example.com"))
	require.Equal(t, common.SignupCreditQuota, getUserQuotaForPaymentGuardTest(t, first.Id))
	require.NoError(t, GrantSignupCredits(first.Id, "email_verified", "email:verified@example.com"))
	require.Equal(t, common.SignupCreditQuota, getUserQuotaForPaymentGuardTest(t, first.Id))

	err := GrantSignupCredits(second.Id, "oauth:oidc", "email:verified@example.com")
	require.ErrorIs(t, err, ErrCreditGrantIdentityUsed)
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, second.Id))
}

func TestSignupCreditIdentityReuseDoesNotRollbackAccountCreation(t *testing.T) {
	truncateTables(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	first := &User{
		Username:             "credits-signup-existing",
		Status:               common.UserStatusEnabled,
		AffCode:              "cs03",
		SignupCreditSource:   "oauth:OIDC",
		SignupCreditIdentity: "oauth:OIDC:shared-subject",
	}
	require.NoError(t, first.Insert(0))
	require.Equal(t, common.SignupCreditQuota, getUserQuotaForPaymentGuardTest(t, first.Id))

	second := &User{
		Username:             "credits-signup-second",
		Status:               common.UserStatusEnabled,
		AffCode:              "cs04",
		SignupCreditSource:   "oauth:OIDC",
		SignupCreditIdentity: "oauth:OIDC:shared-subject",
	}
	require.NoError(t, second.Insert(0))
	require.NotZero(t, second.Id)
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, second.Id))
}

func TestCreditsSignupGrantReplacesLegacyNewUserQuota(t *testing.T) {
	truncateTables(t)
	t.Setenv(common.CreditsFeatureFlagEnv, "true")
	originalQuota := common.QuotaForNewUser
	common.QuotaForNewUser = 999_999
	t.Cleanup(func() {
		common.QuotaForNewUser = originalQuota
	})

	eligible := &User{
		Username:             "credits-signup-replaces-legacy",
		Status:               common.UserStatusEnabled,
		SignupCreditSource:   "email_verified",
		SignupCreditIdentity: "email:replacement@example.com",
	}
	require.NoError(t, eligible.Insert(0))
	require.Equal(t, common.SignupCreditQuota, getUserQuotaForPaymentGuardTest(t, eligible.Id))

	ineligible := &User{
		Username: "credits-signup-no-verification",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, ineligible.Insert(0))
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, ineligible.Id))
}

func TestCompleteCreditsWaffoPancakeValidatesSnapshotAndEventIdempotency(t *testing.T) {
	truncateTables(t)
	userID := 8301
	insertUserForPaymentGuardTest(t, userID, 7)
	pkg, ok := FindCreditPackage("credits-usd-1000")
	require.True(t, ok)
	topUp := &TopUp{
		UserId:             userID,
		Amount:             pkg.Credits,
		Money:              5,
		TradeNo:            "credits-waffo-snapshot",
		PaymentMethod:      PaymentMethodWaffoPancake,
		PaymentProvider:    PaymentProviderWaffoPancake,
		CreateTime:         time.Now().Unix(),
		Status:             common.TopUpStatusPending,
		PackageId:          pkg.PackageId,
		Credits:            pkg.Credits,
		QuotaAmount:        pkg.Quota,
		Currency:           pkg.Currency,
		PriceMinor:         pkg.PriceMinor,
		BaseCredits:        pkg.BaseCredits,
		BonusCredits:       pkg.BonusCredits,
		PaymentStoreId:     "store-prod",
		PaymentEnvironment: "prod",
	}
	require.NoError(t, topUp.Insert())

	err := CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-bad-money", "store-prod", "prod", "USD", 499)
	require.ErrorIs(t, err, ErrPaymentSnapshotMismatch)
	require.Equal(t, 7, getUserQuotaForPaymentGuardTest(t, userID))
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

	err = CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-bad-currency", "store-prod", "prod", "CNY", 500)
	require.ErrorIs(t, err, ErrPaymentSnapshotMismatch)
	require.Equal(t, 7, getUserQuotaForPaymentGuardTest(t, userID))

	err = CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-bad-store", "store-test", "prod", "USD", 500)
	require.ErrorIs(t, err, ErrPaymentSnapshotMismatch)
	require.Equal(t, 7, getUserQuotaForPaymentGuardTest(t, userID))

	err = CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-bad-env", "store-prod", "test", "USD", 500)
	require.ErrorIs(t, err, ErrPaymentSnapshotMismatch)
	require.Equal(t, 7, getUserQuotaForPaymentGuardTest(t, userID))

	require.NoError(t, CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-success", "store-prod", "prod", "USD", 500))
	require.Equal(t, 7+pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))
	require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

	err = CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-success", "store-prod", "prod", "USD", 500)
	require.True(t, errors.Is(err, ErrPaymentEventDuplicate))
	require.Equal(t, 7+pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))

	conflictingTopUp := creditTopUpForTest(userID, "credits-waffo-event-conflict", pkg)
	require.NoError(t, conflictingTopUp.Insert())
	err = CompleteCreditsWaffoPancake(conflictingTopUp.TradeNo, "evt-success", "store-prod", "prod", "USD", 500)
	require.ErrorIs(t, err, ErrPaymentEventConflict)
	require.Equal(t, common.TopUpStatusManualReview, getTopUpStatusForPaymentGuardTest(t, conflictingTopUp.TradeNo))
	require.Equal(t, 7+pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))

	successfulTarget := creditTopUpForTest(userID, "credits-waffo-success-conflict", pkg)
	require.NoError(t, successfulTarget.Insert())
	require.NoError(t, CompleteCreditsWaffoPancake(successfulTarget.TradeNo, "evt-success-target", "store-prod", "prod", "USD", 500))
	quotaAfterSuccessfulTarget := getUserQuotaForPaymentGuardTest(t, userID)
	err = CompleteCreditsWaffoPancake(successfulTarget.TradeNo, "evt-success", "store-prod", "prod", "USD", 500)
	require.ErrorIs(t, err, ErrPaymentEventConflict)
	require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, successfulTarget.TradeNo))
	require.NoError(t, ManualCompleteTopUp(successfulTarget.TradeNo, "127.0.0.1"))
	require.Equal(t, quotaAfterSuccessfulTarget, getUserQuotaForPaymentGuardTest(t, userID))

	err = CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-second", "store-prod", "prod", "USD", 500)
	require.ErrorIs(t, err, ErrTopUpStatusInvalid)
	require.Equal(t, quotaAfterSuccessfulTarget, getUserQuotaForPaymentGuardTest(t, userID))
}

func TestCompleteCreditsTopUpEnforcesBalanceLimit(t *testing.T) {
	pkg, ok := FindCreditPackage("credits-usd-275000")
	require.True(t, ok)

	t.Run("exact boundary succeeds", func(t *testing.T) {
		truncateTables(t)
		userID := 8401
		insertUserForPaymentGuardTest(t, userID, common.MaxQuota-pkg.Quota)
		topUp := creditTopUpForTest(userID, "credits-limit-exact", pkg)
		require.NoError(t, topUp.Insert())
		require.NoError(t, CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-limit-exact", "store-prod", "prod", "USD", pkg.PriceMinor))
		require.Equal(t, common.MaxQuota, getUserQuotaForPaymentGuardTest(t, userID))
	})

	t.Run("one quota over enters manual review atomically", func(t *testing.T) {
		truncateTables(t)
		userID := 8402
		startingQuota := common.MaxQuota - pkg.Quota + 1
		insertUserForPaymentGuardTest(t, userID, startingQuota)
		topUp := creditTopUpForTest(userID, "credits-limit-over", pkg)
		require.NoError(t, topUp.Insert())
		err := CompleteCreditsWaffoPancake(topUp.TradeNo, "evt-limit-over", "store-prod", "prod", "USD", pkg.PriceMinor)
		require.ErrorIs(t, err, ErrCreditsPaymentManualReview)
		require.Equal(t, startingQuota, getUserQuotaForPaymentGuardTest(t, userID))
		require.Equal(t, common.TopUpStatusManualReview, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
	})
}

func TestCreditCheckoutReservationsPreventPendingOvercommit(t *testing.T) {
	truncateTables(t)
	userID := 8450
	insertUserForPaymentGuardTest(t, userID, 200_000_000)
	pkg, ok := FindCreditPackage("credits-usd-275000")
	require.True(t, ok)

	first := creditTopUpForTest(userID, "credits-reservation-first", pkg)
	require.NoError(t, InsertCreditTopUpWithCapacity(first))
	second := creditTopUpForTest(userID, "credits-reservation-second", pkg)
	err := InsertCreditTopUpWithCapacity(second)
	require.ErrorIs(t, err, ErrCreditBalanceLimit)

	var pending int64
	require.NoError(t, DB.Model(&TopUp{}).
		Where("user_id = ? AND status = ?", userID, common.TopUpStatusPending).
		Count(&pending).Error)
	require.EqualValues(t, 1, pending)
	require.Equal(t, 200_000_000, getUserQuotaForPaymentGuardTest(t, userID))
}

func TestCreditCheckoutReservationTTLReleasesCapacity(t *testing.T) {
	truncateTables(t)
	userID := 8451
	insertUserForPaymentGuardTest(t, userID, 200_000_000)
	pkg, ok := FindCreditPackage("credits-usd-275000")
	require.True(t, ok)

	stale := creditTopUpForTest(userID, "credits-reservation-stale", pkg)
	stale.CreateTime = time.Now().Unix() - CreditsCheckoutReservationTTLSeconds - 1
	require.NoError(t, stale.Insert())

	replacement := creditTopUpForTest(userID, "credits-reservation-replacement", pkg)
	require.NoError(t, InsertCreditTopUpWithCapacity(replacement))
	require.Equal(t, common.TopUpStatusExpired, getTopUpStatusForPaymentGuardTest(t, stale.TradeNo))
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, replacement.TradeNo))
}

func TestLateCreditsPaymentCallbackCreditsOrEntersManualReview(t *testing.T) {
	pkg, ok := FindCreditPackage("credits-usd-275000")
	require.True(t, ok)

	t.Run("expired reservation still credits a valid paid order", func(t *testing.T) {
		truncateTables(t)
		userID := 8452
		insertUserForPaymentGuardTest(t, userID, 10)
		topUp := creditTopUpForTest(userID, "credits-late-payment-success", pkg)
		topUp.CreateTime = time.Now().Unix() - CreditsCheckoutReservationTTLSeconds - 1
		topUp.Status = common.TopUpStatusExpired
		require.NoError(t, topUp.Insert())

		require.NoError(t, CompleteCreditsWaffoPancake(
			topUp.TradeNo, "evt-late-success", "store-prod", "prod", "USD", pkg.PriceMinor,
		))
		require.Equal(t, 10+pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))
		require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
	})

	t.Run("valid payment that cannot fit is held for manual review", func(t *testing.T) {
		truncateTables(t)
		userID := 8453
		startingQuota := common.MaxQuota - pkg.Quota + 1
		insertUserForPaymentGuardTest(t, userID, startingQuota)
		topUp := creditTopUpForTest(userID, "credits-late-payment-review", pkg)
		topUp.CreateTime = time.Now().Unix() - CreditsCheckoutReservationTTLSeconds - 1
		topUp.Status = common.TopUpStatusExpired
		require.NoError(t, topUp.Insert())

		err := CompleteCreditsWaffoPancake(
			topUp.TradeNo, "evt-late-review", "store-prod", "prod", "USD", pkg.PriceMinor,
		)
		require.ErrorIs(t, err, ErrCreditsPaymentManualReview)
		require.Equal(t, startingQuota, getUserQuotaForPaymentGuardTest(t, userID))
		require.Equal(t, common.TopUpStatusManualReview, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

		err = CompleteCreditsWaffoPancake(
			topUp.TradeNo, "evt-late-review", "store-prod", "prod", "USD", pkg.PriceMinor,
		)
		require.ErrorIs(t, err, ErrPaymentEventDuplicate)
		var events int64
		require.NoError(t, DB.Model(&PaymentEvent{}).
			Where("provider = ? AND event_id = ?", PaymentProviderWaffoPancake, "evt-late-review").
			Count(&events).Error)
		require.EqualValues(t, 1, events)
	})
}

func TestManualCompleteCreditsUsesQuotaSnapshot(t *testing.T) {
	truncateTables(t)
	userID := 8501
	insertUserForPaymentGuardTest(t, userID, 11)
	pkg, ok := FindCreditPackage("credits-cny-1000")
	require.True(t, ok)
	topUp := creditTopUpForTest(userID, "credits-manual-snapshot", pkg)
	topUp.PaymentMethod = "alipay"
	topUp.PaymentProvider = PaymentProviderEpay
	topUp.PaymentStoreId = "merchant"
	require.NoError(t, topUp.Insert())

	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))
	require.Equal(t, 11+pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))
	require.NotEqual(t, 11+int(pkg.Credits)*int(common.QuotaPerUnit), getUserQuotaForPaymentGuardTest(t, userID))
}

func TestManualCompleteCreditsResolvesCapacityReview(t *testing.T) {
	truncateTables(t)
	userID := 8502
	pkg, ok := FindCreditPackage("credits-usd-275000")
	require.True(t, ok)
	startingQuota := common.MaxQuota - pkg.Quota + 1
	insertUserForPaymentGuardTest(t, userID, startingQuota)
	topUp := creditTopUpForTest(userID, "credits-manual-review-resolution", pkg)
	topUp.Status = common.TopUpStatusManualReview
	require.NoError(t, topUp.Insert())

	err := ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1")
	require.ErrorIs(t, err, ErrCreditBalanceLimit)
	require.Equal(t, startingQuota, getUserQuotaForPaymentGuardTest(t, userID))
	require.Equal(t, common.TopUpStatusManualReview, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))

	require.NoError(t, DB.Model(&User{}).Where("id = ?", userID).Update("quota", startingQuota-1).Error)
	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))
	require.Equal(t, common.MaxQuota, getUserQuotaForPaymentGuardTest(t, userID))
	require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, topUp.TradeNo))
}

func TestCompleteCreditsEpayValidatesMerchantAndPaymentMethod(t *testing.T) {
	truncateTables(t)
	userID := 8601
	insertUserForPaymentGuardTest(t, userID, 0)
	pkg, ok := FindCreditPackage("credits-cny-1000")
	require.True(t, ok)
	topUp := creditTopUpForTest(userID, "credits-epay-snapshot", pkg)
	topUp.PaymentMethod = "alipay"
	topUp.PaymentProvider = PaymentProviderEpay
	topUp.PaymentStoreId = "merchant-a"
	require.NoError(t, topUp.Insert())

	err := CompleteCreditsEpay(topUp.TradeNo, "evt-wrong-merchant", "merchant-b", "alipay", "CNY", pkg.PriceMinor)
	require.ErrorIs(t, err, ErrPaymentSnapshotMismatch)
	err = CompleteCreditsEpay(topUp.TradeNo, "evt-wrong-method", "merchant-a", "wxpay", "CNY", pkg.PriceMinor)
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)
	require.Zero(t, getUserQuotaForPaymentGuardTest(t, userID))

	require.NoError(t, CompleteCreditsEpay(topUp.TradeNo, "evt-epay-ok", "merchant-a", "alipay", "CNY", pkg.PriceMinor))
	require.Equal(t, pkg.Quota, getUserQuotaForPaymentGuardTest(t, userID))
}

func creditTopUpForTest(userID int, tradeNo string, pkg CreditPackage) *TopUp {
	return &TopUp{
		UserId:             userID,
		Amount:             pkg.Credits,
		Money:              float64(pkg.PriceMinor) / 100,
		TradeNo:            tradeNo,
		PaymentMethod:      PaymentMethodWaffoPancake,
		PaymentProvider:    PaymentProviderWaffoPancake,
		CreateTime:         time.Now().Unix(),
		Status:             common.TopUpStatusPending,
		PackageId:          pkg.PackageId,
		Credits:            pkg.Credits,
		QuotaAmount:        pkg.Quota,
		Currency:           pkg.Currency,
		PriceMinor:         pkg.PriceMinor,
		BaseCredits:        pkg.BaseCredits,
		BonusCredits:       pkg.BonusCredits,
		PaymentStoreId:     "store-prod",
		PaymentEnvironment: "prod",
	}
}
