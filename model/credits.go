package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const SignupCreditGrantType = "signup_credits_v1"

const CreditsCheckoutReservationTTLSeconds int64 = 45 * 60

var (
	ErrCreditGrantDuplicate       = errors.New("credit grant already applied")
	ErrCreditGrantIdentityUsed    = errors.New("credit grant identity already used")
	ErrPaymentEventDuplicate      = errors.New("payment event already processed")
	ErrPaymentSnapshotMismatch    = errors.New("payment snapshot mismatch")
	ErrCreditsCheckoutNotEnabled  = errors.New("credits checkout is not enabled")
	ErrCreditBalanceLimit         = errors.New("credit balance limit exceeded")
	ErrCreditsPaymentManualReview = errors.New("credits payment requires manual review")
)

type CreditGrant struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"uniqueIndex:idx_credit_grant_user_type;index"`
	GrantType    string `json:"grant_type" gorm:"type:varchar(64);uniqueIndex:idx_credit_grant_user_type"`
	Source       string `json:"source" gorm:"type:varchar(32)"`
	IdentityHash string `json:"-" gorm:"type:char(64);uniqueIndex"`
	Credits      int64  `json:"credits"`
	Quota        int    `json:"quota" gorm:"type:bigint"`
	CreatedAt    int64  `json:"created_at"`
}

type PaymentEvent struct {
	Id        int    `json:"id"`
	Provider  string `json:"provider" gorm:"type:varchar(50);uniqueIndex:idx_payment_event_provider_id"`
	EventId   string `json:"event_id" gorm:"type:varchar(191);uniqueIndex:idx_payment_event_provider_id"`
	TradeNo   string `json:"trade_no" gorm:"type:varchar(255);index"`
	CreatedAt int64  `json:"created_at"`
}

func signupCreditIdentityHash(identity string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(identity)))))
}

func grantSignupCreditsWithTx(tx *gorm.DB, user *User) error {
	if user == nil || user.Id == 0 || !common.CreditsV1Enabled() {
		return nil
	}
	source := strings.TrimSpace(user.SignupCreditSource)
	identity := strings.TrimSpace(user.SignupCreditIdentity)
	if source == "" || identity == "" {
		return nil
	}
	grant := CreditGrant{
		UserId:       user.Id,
		GrantType:    SignupCreditGrantType,
		Source:       source,
		IdentityHash: signupCreditIdentityHash(identity),
		Credits:      common.SignupCreditAmount,
		Quota:        common.SignupCreditQuota,
		CreatedAt:    time.Now().Unix(),
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&grant)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var existing CreditGrant
		if err := tx.Where("user_id = ? AND grant_type = ?", user.Id, SignupCreditGrantType).First(&existing).Error; err == nil {
			return nil
		}
		if err := tx.Where("identity_hash = ?", grant.IdentityHash).First(&existing).Error; err == nil {
			return ErrCreditGrantIdentityUsed
		}
		return ErrCreditGrantDuplicate
	}
	return tx.Model(&User{}).Where("id = ?", user.Id).
		Update("quota", gorm.Expr("quota + ?", common.SignupCreditQuota)).Error
}

// GrantSignupCredits is exposed for retry/reconciliation jobs. It is
// idempotent by both user+grant type and hashed verified identity.
func GrantSignupCredits(userID int, source, identity string) error {
	if !common.CreditsV1Enabled() {
		return nil
	}
	user := &User{Id: userID, SignupCreditSource: source, SignupCreditIdentity: identity}
	return DB.Transaction(func(tx *gorm.DB) error {
		return grantSignupCreditsWithTx(tx, user)
	})
}

func validateCreditTopUpCapacity(currentQuota, reservedQuota, quotaToAdd int) error {
	if quotaToAdd <= 0 || currentQuota < 0 || reservedQuota < 0 ||
		reservedQuota > common.MaxQuota-quotaToAdd ||
		currentQuota > common.MaxQuota-reservedQuota-quotaToAdd {
		return ErrCreditBalanceLimit
	}
	return nil
}

func pendingCreditTopUpQuota(tx *gorm.DB, userID int) (int, error) {
	cutoff := time.Now().Unix() - CreditsCheckoutReservationTTLSeconds
	if err := tx.Model(&TopUp{}).
		Where("user_id = ? AND status = ? AND package_id <> '' AND create_time <= ?",
			userID, common.TopUpStatusPending, cutoff).
		Update("status", common.TopUpStatusExpired).Error; err != nil {
		return 0, err
	}
	var reserved int64
	if err := tx.Model(&TopUp{}).
		Where("user_id = ? AND status = ? AND package_id <> ''", userID, common.TopUpStatusPending).
		Select("COALESCE(SUM(quota_amount), 0)").
		Scan(&reserved).Error; err != nil {
		return 0, err
	}
	if reserved < 0 || reserved > int64(common.MaxQuota) {
		return 0, ErrCreditBalanceLimit
	}
	return int(reserved), nil
}

func checkCreditTopUpCapacity(tx *gorm.DB, userID, quotaToAdd int, includePending bool) error {
	var user User
	if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	reserved := 0
	if includePending {
		var err error
		reserved, err = pendingCreditTopUpQuota(tx, userID)
		if err != nil {
			return err
		}
	}
	return validateCreditTopUpCapacity(user.Quota, reserved, quotaToAdd)
}

// CheckCreditTopUpCapacity is a read-only checkout preflight. The atomic
// reservation below repeats the check while holding the user row lock.
func CheckCreditTopUpCapacity(userID, quotaToAdd int) error {
	return checkCreditTopUpCapacity(DB, userID, quotaToAdd, true)
}

// InsertCreditTopUpWithCapacity reserves quota with the pending order. The
// user row lock serializes concurrent checkouts for the same account.
func InsertCreditTopUpWithCapacity(topUp *TopUp) error {
	if topUp == nil || topUp.PackageId == "" {
		return ErrPaymentSnapshotMismatch
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := checkCreditTopUpCapacity(tx, topUp.UserId, topUp.QuotaAmount, true); err != nil {
			return err
		}
		return tx.Create(topUp).Error
	})
}

func addCreditTopUpQuota(tx *gorm.DB, userID, quotaToAdd int) error {
	if err := checkCreditTopUpCapacity(tx, userID, quotaToAdd, false); err != nil {
		return err
	}
	result := tx.Model(&User{}).Where("id = ?", userID).
		Update("quota", gorm.Expr("quota + ?", quotaToAdd))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
