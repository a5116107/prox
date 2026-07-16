package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	UserQuotaReservationActive   = "active"
	UserQuotaReservationSettled  = "settled"
	UserQuotaReservationRefunded = "refunded"

	userQuotaReservationTTL = 2 * time.Hour
)

var (
	ErrUserQuotaInsufficient        = errors.New("user quota insufficient")
	ErrUserQuotaReservationConflict = errors.New("user quota reservation conflict")
	ErrUserQuotaReservationState    = errors.New("user quota reservation state conflict")
	ErrUserQuotaMutationConflict    = errors.New("user quota mutation idempotency conflict")
)

// UserQuotaReservation is the durable wallet hold for one billable request.
// RequestId is globally unique so retries across nodes converge on one row.
type UserQuotaReservation struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	RequestId     string `json:"request_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_user_quota_reservation_request"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	SourceType    string `json:"source_type" gorm:"type:varchar(64);not null;default:'api_billing';index"`
	InitialQuota  int    `json:"initial_quota" gorm:"not null;default:0"`
	ReservedQuota int    `json:"reserved_quota" gorm:"not null;default:0"`
	SettledQuota  int    `json:"settled_quota" gorm:"not null;default:0"`
	Status        string `json:"status" gorm:"type:varchar(24);not null;default:'active';index"`
	BalanceAfter  int    `json:"balance_after" gorm:"not null;default:0"`
	Version       int    `json:"version" gorm:"not null;default:1"`
	ExpiresAt     int64  `json:"expires_at" gorm:"not null;default:0;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (UserQuotaReservation) TableName() string { return "user_quota_reservations" }

// UserQuotaTransaction is an immutable wallet mutation journal. DeltaQuota is
// signed from the user's perspective: debits are negative and credits positive.
type UserQuotaTransaction struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"not null;index"`
	ReservationId   int    `json:"reservation_id" gorm:"index"`
	RequestId       string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	SourceType      string `json:"source_type" gorm:"type:varchar(64);not null;default:'';index"`
	TransactionType string `json:"transaction_type" gorm:"type:varchar(32);not null;index"`
	DeltaQuota      int    `json:"delta_quota" gorm:"not null"`
	BalanceBefore   int    `json:"balance_before" gorm:"not null"`
	BalanceAfter    int    `json:"balance_after" gorm:"not null"`
	IdempotencyKey  string `json:"idempotency_key" gorm:"type:varchar(191);not null;uniqueIndex:ux_user_quota_txn_idem"`
	Remark          string `json:"remark" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (UserQuotaTransaction) TableName() string { return "user_quota_transactions" }

type UserQuotaReconciliationReport struct {
	NegativeUsers               int64 `json:"negative_users"`
	ActiveReservations          int64 `json:"active_reservations"`
	ExpiredActiveReservations   int64 `json:"expired_active_reservations"`
	OrphanReservations          int64 `json:"orphan_reservations"`
	ReservationLedgerMismatches int64 `json:"reservation_ledger_mismatches"`
}

type UserQuotaMutation struct {
	UserID          int
	DeltaQuota      int
	RequestID       string
	SourceType      string
	TransactionType string
	IdempotencyKey  string
	Remark          string
}

func normalizeUserQuotaRequest(requestID string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return "", errors.New("wallet request id must contain 1 to 128 characters")
	}
	return requestID, nil
}

func normalizeUserQuotaSource(sourceType string) (string, error) {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		return "api_billing", nil
	}
	if len(sourceType) > 64 {
		return "", errors.New("wallet source type cannot exceed 64 characters")
	}
	return sourceType, nil
}

func invalidateUserQuotaCache(userID int, operation string) {
	if err := invalidateUserCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user quota cache after %s for user %d: %s", operation, userID, err.Error()))
	}
}

func lockUserQuotaTx(tx *gorm.DB, userID int) (*User, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	var user User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func debitUserQuotaTx(tx *gorm.DB, userID int, quota int) (int, int, error) {
	if userID <= 0 || quota < 0 {
		return 0, 0, errors.New("user id must be positive and quota cannot be negative")
	}
	user, err := lockUserQuotaTx(tx, userID)
	if err != nil {
		return 0, 0, err
	}
	if quota == 0 {
		return user.Quota, user.Quota, nil
	}
	if user.Quota < quota {
		return user.Quota, user.Quota, fmt.Errorf("%w: balance=%d required=%d user_id=%d", ErrUserQuotaInsufficient, user.Quota, quota, userID)
	}
	debit := tx.Model(&User{}).
		Where("id = ? AND quota >= ?", userID, quota).
		Update("quota", gorm.Expr("quota - ?", quota))
	if debit.Error != nil {
		return 0, 0, debit.Error
	}
	if debit.RowsAffected != 1 {
		return user.Quota, user.Quota, fmt.Errorf("%w: concurrent debit user_id=%d required=%d", ErrUserQuotaInsufficient, userID, quota)
	}
	return user.Quota, user.Quota - quota, nil
}

func creditUserQuotaTx(tx *gorm.DB, userID int, quota int) (int, int, error) {
	if userID <= 0 || quota < 0 {
		return 0, 0, errors.New("user id must be positive and quota cannot be negative")
	}
	user, err := lockUserQuotaTx(tx, userID)
	if err != nil {
		return 0, 0, err
	}
	if quota == 0 {
		return user.Quota, user.Quota, nil
	}
	credit := tx.Model(&User{}).Where("id = ?", userID).
		Update("quota", gorm.Expr("quota + ?", quota))
	if credit.Error != nil {
		return 0, 0, credit.Error
	}
	if credit.RowsAffected != 1 {
		return user.Quota, user.Quota, fmt.Errorf("credit user quota affected %d rows: user_id=%d", credit.RowsAffected, userID)
	}
	return user.Quota, user.Quota + quota, nil
}

func createUserQuotaTransactionTx(tx *gorm.DB, row *UserQuotaTransaction) error {
	if row == nil {
		return errors.New("user quota transaction is nil")
	}
	return tx.Create(row).Error
}

func normalizeUserQuotaMutation(mutation UserQuotaMutation) (UserQuotaMutation, error) {
	if mutation.UserID <= 0 {
		return mutation, errors.New("user id must be positive")
	}
	requestID, err := normalizeUserQuotaRequest(mutation.RequestID)
	if err != nil {
		return mutation, err
	}
	mutation.RequestID = requestID
	mutation.SourceType, err = normalizeUserQuotaSource(mutation.SourceType)
	if err != nil {
		return mutation, err
	}
	mutation.TransactionType = strings.TrimSpace(mutation.TransactionType)
	if mutation.TransactionType == "" || len(mutation.TransactionType) > 32 {
		return mutation, errors.New("wallet transaction type must contain 1 to 32 characters")
	}
	mutation.IdempotencyKey = strings.TrimSpace(mutation.IdempotencyKey)
	if mutation.IdempotencyKey == "" || len(mutation.IdempotencyKey) > 191 {
		return mutation, errors.New("wallet idempotency key must contain 1 to 191 characters")
	}
	mutation.Remark = strings.TrimSpace(mutation.Remark)
	return mutation, nil
}

func validateUserQuotaMutationReplay(existing *UserQuotaTransaction, mutation UserQuotaMutation) error {
	if existing == nil ||
		existing.UserId != mutation.UserID ||
		existing.RequestId != mutation.RequestID ||
		existing.SourceType != mutation.SourceType ||
		existing.TransactionType != mutation.TransactionType ||
		existing.DeltaQuota != mutation.DeltaQuota {
		return fmt.Errorf("%w: idempotency_key=%s", ErrUserQuotaMutationConflict, mutation.IdempotencyKey)
	}
	return nil
}

// ApplyUserQuotaMutationTx applies one signed wallet mutation exactly once.
// The journal row claims the idempotency key before the balance is changed;
// the same database transaction then records the resulting balance snapshot.
func applyUserQuotaMutationTx(tx *gorm.DB, mutation UserQuotaMutation) (bool, error) {
	if tx == nil {
		return false, errors.New("db transaction is nil")
	}
	mutation, err := normalizeUserQuotaMutation(mutation)
	if err != nil {
		return false, err
	}

	journal := UserQuotaTransaction{
		UserId: mutation.UserID, RequestId: mutation.RequestID,
		SourceType: mutation.SourceType, TransactionType: mutation.TransactionType,
		DeltaQuota: mutation.DeltaQuota, IdempotencyKey: mutation.IdempotencyKey,
		Remark: mutation.Remark,
	}
	claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&journal)
	if claim.Error != nil {
		return false, claim.Error
	}
	if claim.RowsAffected == 0 {
		var existing UserQuotaTransaction
		if err := tx.Where("idempotency_key = ?", mutation.IdempotencyKey).First(&existing).Error; err != nil {
			return false, err
		}
		return false, validateUserQuotaMutationReplay(&existing, mutation)
	}

	var before, after int
	switch {
	case mutation.DeltaQuota < 0:
		before, after, err = debitUserQuotaTx(tx, mutation.UserID, -mutation.DeltaQuota)
	case mutation.DeltaQuota > 0:
		before, after, err = creditUserQuotaTx(tx, mutation.UserID, mutation.DeltaQuota)
	default:
		var user *User
		user, err = lockUserQuotaTx(tx, mutation.UserID)
		if err == nil {
			before, after = user.Quota, user.Quota
		}
	}
	if err != nil {
		return false, err
	}
	finalize := tx.Model(&UserQuotaTransaction{}).Where("id = ?", journal.Id).
		Updates(map[string]any{"balance_before": before, "balance_after": after})
	if finalize.Error != nil {
		return false, finalize.Error
	}
	if finalize.RowsAffected != 1 {
		return false, fmt.Errorf("finalize wallet journal affected %d rows: idempotency_key=%s", finalize.RowsAffected, mutation.IdempotencyKey)
	}
	return true, nil
}

func ApplyUserQuotaMutationTx(tx *gorm.DB, mutation UserQuotaMutation) error {
	_, err := applyUserQuotaMutationTx(tx, mutation)
	return err
}

func ApplyUserQuotaMutation(mutation UserQuotaMutation) error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := applyUserQuotaMutationTx(tx, mutation)
		return err
	})
	if err == nil {
		invalidateUserQuotaCache(mutation.UserID, mutation.TransactionType)
	}
	return err
}

func ApplyUserQuotaMutationWithResult(mutation UserQuotaMutation) (bool, error) {
	applied := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		applied, err = applyUserQuotaMutationTx(tx, mutation)
		return err
	})
	if err == nil && applied {
		invalidateUserQuotaCache(mutation.UserID, mutation.TransactionType)
	}
	return applied, err
}

func SetUserQuotaBalance(userID int, targetQuota int, requestID string, sourceType string, remark string) error {
	if userID <= 0 || targetQuota < 0 {
		return errors.New("user id must be positive and target quota cannot be negative")
	}
	requestID, err := normalizeUserQuotaRequest(requestID)
	if err != nil {
		return err
	}
	sourceType, err = normalizeUserQuotaSource(sourceType)
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		user, err := lockUserQuotaTx(tx, userID)
		if err != nil {
			return err
		}
		var replay UserQuotaTransaction
		replayErr := tx.Where("idempotency_key = ?", requestID).First(&replay).Error
		if replayErr == nil {
			if replay.UserId == userID && replay.SourceType == sourceType && replay.TransactionType == "quota_override" && replay.BalanceAfter == targetQuota {
				return nil
			}
			return fmt.Errorf("%w: idempotency_key=%s", ErrUserQuotaMutationConflict, requestID)
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}
		return ApplyUserQuotaMutationTx(tx, UserQuotaMutation{
			UserID: userID, DeltaQuota: targetQuota - user.Quota, RequestID: requestID,
			SourceType: sourceType, TransactionType: "quota_override",
			IdempotencyKey: requestID, Remark: strings.TrimSpace(remark),
		})
	})
	if err == nil {
		invalidateUserQuotaCache(userID, "quota override")
	}
	return err
}

func lockUserQuotaReservationTx(tx *gorm.DB, requestID string) (*UserQuotaReservation, error) {
	var reservation UserQuotaReservation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("request_id = ?", requestID).First(&reservation).Error; err != nil {
		return nil, err
	}
	return &reservation, nil
}

func validateUserQuotaReservationReplay(reservation *UserQuotaReservation, userID int, initialQuota int, sourceType string) error {
	if reservation.UserId != userID || reservation.InitialQuota != initialQuota || reservation.SourceType != sourceType {
		return fmt.Errorf("%w: request_id=%s", ErrUserQuotaReservationConflict, reservation.RequestId)
	}
	if reservation.Status != UserQuotaReservationActive {
		return fmt.Errorf("%w: status=%s request_id=%s", ErrUserQuotaReservationState, reservation.Status, reservation.RequestId)
	}
	return nil
}

func ReserveUserQuota(requestID string, userID int, quota int, sourceType string) error {
	requestID, err := normalizeUserQuotaRequest(requestID)
	if err != nil {
		return err
	}
	if userID <= 0 || quota <= 0 {
		return errors.New("user id and reservation quota must be positive")
	}
	sourceType, err = normalizeUserQuotaSource(sourceType)
	if err != nil {
		return err
	}
	changed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		reservation := UserQuotaReservation{
			RequestId: requestID, UserId: userID, SourceType: sourceType,
			InitialQuota: quota, ReservedQuota: quota, Status: UserQuotaReservationActive,
			Version: 1, ExpiresAt: now.Add(userQuotaReservationTTL).Unix(),
		}
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 0 {
			existing, lockErr := lockUserQuotaReservationTx(tx, requestID)
			if lockErr != nil {
				return lockErr
			}
			return validateUserQuotaReservationReplay(existing, userID, quota, sourceType)
		}

		before, after, debitErr := debitUserQuotaTx(tx, userID, quota)
		if debitErr != nil {
			return debitErr
		}
		if err := tx.Model(&UserQuotaReservation{}).Where("id = ?", reservation.Id).
			Update("balance_after", after).Error; err != nil {
			return err
		}
		if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
			UserId: userID, ReservationId: reservation.Id, RequestId: requestID,
			SourceType: sourceType, TransactionType: "reserve", DeltaQuota: -quota,
			BalanceBefore: before, BalanceAfter: after,
			IdempotencyKey: requestID + ":reserve", Remark: "wallet quota reserved",
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err == nil && changed {
		invalidateUserQuotaCache(userID, "reservation")
	}
	return err
}

// ResizeUserQuotaReservation changes an active hold to an absolute target.
// Repeating the same target is a no-op, while reductions return quota.
func ResizeUserQuotaReservation(requestID string, userID int, targetQuota int) error {
	requestID, err := normalizeUserQuotaRequest(requestID)
	if err != nil {
		return err
	}
	if userID <= 0 || targetQuota < 0 {
		return errors.New("user id must be positive and target quota cannot be negative")
	}
	changed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		reservation, err := lockUserQuotaReservationTx(tx, requestID)
		if err != nil {
			return err
		}
		if reservation.UserId != userID {
			return fmt.Errorf("%w: request_id=%s", ErrUserQuotaReservationConflict, requestID)
		}
		if reservation.Status != UserQuotaReservationActive {
			return fmt.Errorf("%w: status=%s request_id=%s", ErrUserQuotaReservationState, reservation.Status, requestID)
		}
		if reservation.ReservedQuota == targetQuota {
			return nil
		}

		before, after := 0, 0
		delta := targetQuota - reservation.ReservedQuota
		if delta > 0 {
			before, after, err = debitUserQuotaTx(tx, userID, delta)
		} else {
			before, after, err = creditUserQuotaTx(tx, userID, -delta)
		}
		if err != nil {
			return err
		}
		nextVersion := reservation.Version + 1
		update := tx.Model(&UserQuotaReservation{}).
			Where("id = ? AND status = ? AND version = ?", reservation.Id, UserQuotaReservationActive, reservation.Version).
			Updates(map[string]any{
				"reserved_quota": targetQuota, "balance_after": after,
				"version": nextVersion, "expires_at": time.Now().Add(userQuotaReservationTTL).Unix(),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: concurrent resize request_id=%s", ErrUserQuotaReservationState, requestID)
		}
		if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
			UserId: userID, ReservationId: reservation.Id, RequestId: requestID,
			SourceType: reservation.SourceType, TransactionType: "resize", DeltaQuota: -delta,
			BalanceBefore: before, BalanceAfter: after,
			IdempotencyKey: fmt.Sprintf("%s:resize:%d", requestID, nextVersion),
			Remark:         fmt.Sprintf("wallet reservation resized to %d", targetQuota),
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err == nil && changed {
		invalidateUserQuotaCache(userID, "reservation resize")
	}
	return err
}

// SettleUserQuotaReservation commits the absolute quota charged by a request.
// Trusted requests may settle without an earlier hold; that path creates a
// settled reservation and performs one conditional debit in the same transaction.
func SettleUserQuotaReservation(requestID string, userID int, actualQuota int, sourceType string) error {
	requestID, err := normalizeUserQuotaRequest(requestID)
	if err != nil {
		return err
	}
	if userID <= 0 || actualQuota < 0 {
		return errors.New("user id must be positive and actual quota cannot be negative")
	}
	sourceType, err = normalizeUserQuotaSource(sourceType)
	if err != nil {
		return err
	}
	changed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		reservation, err := lockUserQuotaReservationTx(tx, requestID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reservation = &UserQuotaReservation{
				RequestId: requestID, UserId: userID, SourceType: sourceType,
				InitialQuota: 0, ReservedQuota: actualQuota, SettledQuota: actualQuota,
				Status: UserQuotaReservationSettled, Version: 1,
			}
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(reservation)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 0 {
				reservation, err = lockUserQuotaReservationTx(tx, requestID)
				if err != nil {
					return err
				}
			} else {
				before, after, debitErr := debitUserQuotaTx(tx, userID, actualQuota)
				if debitErr != nil {
					return debitErr
				}
				if err := tx.Model(&UserQuotaReservation{}).Where("id = ?", reservation.Id).
					Update("balance_after", after).Error; err != nil {
					return err
				}
				if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
					UserId: userID, ReservationId: reservation.Id, RequestId: requestID,
					SourceType: sourceType, TransactionType: "settle", DeltaQuota: -actualQuota,
					BalanceBefore: before, BalanceAfter: after,
					IdempotencyKey: requestID + ":settle", Remark: "trusted wallet request settled",
				}); err != nil {
					return err
				}
				changed = true
				return nil
			}
		}
		if err != nil {
			return err
		}
		if reservation.UserId != userID || reservation.SourceType != sourceType {
			return fmt.Errorf("%w: request_id=%s", ErrUserQuotaReservationConflict, requestID)
		}
		switch reservation.Status {
		case UserQuotaReservationSettled:
			if reservation.SettledQuota == actualQuota {
				return nil
			}
			return fmt.Errorf("%w: settled=%d requested=%d request_id=%s", ErrUserQuotaReservationConflict, reservation.SettledQuota, actualQuota, requestID)
		case UserQuotaReservationRefunded:
			return fmt.Errorf("%w: reservation already refunded request_id=%s", ErrUserQuotaReservationState, requestID)
		case UserQuotaReservationActive:
		default:
			return fmt.Errorf("%w: unrecognized status=%s request_id=%s", ErrUserQuotaReservationState, reservation.Status, requestID)
		}

		before, after := 0, 0
		userDelta := reservation.ReservedQuota - actualQuota
		if userDelta > 0 {
			before, after, err = creditUserQuotaTx(tx, userID, userDelta)
		} else {
			before, after, err = debitUserQuotaTx(tx, userID, -userDelta)
		}
		if err != nil {
			return err
		}
		nextVersion := reservation.Version + 1
		update := tx.Model(&UserQuotaReservation{}).
			Where("id = ? AND status = ? AND version = ?", reservation.Id, UserQuotaReservationActive, reservation.Version).
			Updates(map[string]any{
				"settled_quota": actualQuota, "status": UserQuotaReservationSettled,
				"balance_after": after, "version": nextVersion, "expires_at": 0,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: concurrent settlement request_id=%s", ErrUserQuotaReservationState, requestID)
		}
		if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
			UserId: userID, ReservationId: reservation.Id, RequestId: requestID,
			SourceType: sourceType, TransactionType: "settle", DeltaQuota: userDelta,
			BalanceBefore: before, BalanceAfter: after,
			IdempotencyKey: requestID + ":settle", Remark: fmt.Sprintf("wallet request settled at %d", actualQuota),
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err == nil && changed {
		invalidateUserQuotaCache(userID, "reservation settlement")
	}
	return err
}

func RefundUserQuotaReservation(requestID string, userID int, remark string) error {
	requestID, err := normalizeUserQuotaRequest(requestID)
	if err != nil {
		return err
	}
	if userID <= 0 {
		return errors.New("user id must be positive")
	}
	changed := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		reservation, err := lockUserQuotaReservationTx(tx, requestID)
		if err != nil {
			return err
		}
		if reservation.UserId != userID {
			return fmt.Errorf("%w: request_id=%s", ErrUserQuotaReservationConflict, requestID)
		}
		if reservation.Status == UserQuotaReservationRefunded {
			return nil
		}
		if reservation.Status != UserQuotaReservationActive {
			return fmt.Errorf("%w: status=%s request_id=%s", ErrUserQuotaReservationState, reservation.Status, requestID)
		}
		before, after, err := creditUserQuotaTx(tx, userID, reservation.ReservedQuota)
		if err != nil {
			return err
		}
		nextVersion := reservation.Version + 1
		update := tx.Model(&UserQuotaReservation{}).
			Where("id = ? AND status = ? AND version = ?", reservation.Id, UserQuotaReservationActive, reservation.Version).
			Updates(map[string]any{
				"status": UserQuotaReservationRefunded, "balance_after": after,
				"version": nextVersion, "expires_at": 0,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: concurrent refund request_id=%s", ErrUserQuotaReservationState, requestID)
		}
		if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
			UserId: userID, ReservationId: reservation.Id, RequestId: requestID,
			SourceType: reservation.SourceType, TransactionType: "refund",
			DeltaQuota: reservation.ReservedQuota, BalanceBefore: before, BalanceAfter: after,
			IdempotencyKey: requestID + ":refund", Remark: strings.TrimSpace(remark),
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err == nil && changed {
		invalidateUserQuotaCache(userID, "reservation refund")
	}
	return err
}
