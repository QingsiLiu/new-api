package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	funnelRawRetentionDays      int64 = 180
	funnelActivityRetentionDays int64 = 730
)

var ErrInvalidFunnelMaintenanceInput = errors.New("invalid funnel maintenance input")

type FunnelMaintenanceResult struct {
	ReconciledActivityDays int64 `json:"reconciled_activity_days"`
	DeletedEvents          int64 `json:"deleted_events"`
	AnonymizedVisitors     int64 `json:"anonymized_visitors"`
	DeletedVisitors        int64 `json:"deleted_visitors"`
	DeletedActivityDays    int64 `json:"deleted_activity_days"`
	RawCutoff              int64 `json:"raw_cutoff"`
	ActivityCutoff         int64 `json:"activity_cutoff"`
}

type funnelActiveEvent struct {
	ID          int64
	Environment string
	UserID      int
	ReceivedAt  int64
}

func RunFunnelMaintenance(ctx context.Context, now int64, batchSize int) (FunnelMaintenanceResult, error) {
	if now <= 0 || batchSize <= 0 {
		return FunnelMaintenanceResult{}, ErrInvalidFunnelMaintenanceInput
	}
	result := FunnelMaintenanceResult{
		RawCutoff:      now - funnelRawRetentionDays*86400,
		ActivityCutoff: utcFunnelDay(now) - funnelActivityRetentionDays*86400,
	}

	var err error
	result.ReconciledActivityDays, err = reconcileFunnelActivityDays(ctx, result.RawCutoff, batchSize)
	if err != nil {
		return result, err
	}
	result.DeletedEvents, err = deleteFunnelEventsBefore(ctx, result.RawCutoff, batchSize)
	if err != nil {
		return result, err
	}
	result.AnonymizedVisitors, err = anonymizeInactiveFunnelVisitors(ctx, result.RawCutoff, batchSize)
	if err != nil {
		return result, err
	}
	deletedUnlinked, err := deleteInactiveUnlinkedFunnelVisitors(ctx, result.RawCutoff, batchSize)
	if err != nil {
		return result, err
	}
	deletedLinked, err := deleteExpiredLinkedFunnelVisitors(ctx, result.ActivityCutoff, batchSize)
	if err != nil {
		return result, err
	}
	result.DeletedVisitors = deletedUnlinked + deletedLinked
	result.DeletedActivityDays, err = deleteFunnelActivityDaysBefore(ctx, result.ActivityCutoff, batchSize)
	return result, err
}

func reconcileFunnelActivityDays(ctx context.Context, rawCutoff int64, batchSize int) (int64, error) {
	var reconciled int64
	var lastID int64
	for {
		if err := ctx.Err(); err != nil {
			return reconciled, err
		}
		rows := make([]funnelActiveEvent, 0, batchSize)
		err := DB.WithContext(ctx).Table("funnel_events AS events").
			Select("events.id, events.environment, visitors.user_id AS user_id, events.received_at").
			Joins("JOIN funnel_visitors AS visitors ON visitors.id = events.visitor_id AND visitors.environment = events.environment").
			Where("events.id > ? AND events.event_name = ? AND events.received_at >= ? AND visitors.identity_state = ? AND visitors.user_id IS NOT NULL", lastID, FunnelEventAccountActive, rawCutoff, FunnelIdentityLinked).
			Order("events.id ASC").Limit(batchSize).Scan(&rows).Error
		if err != nil {
			return reconciled, err
		}
		if len(rows) == 0 {
			return reconciled, nil
		}
		var batchReconciled int64
		if err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, row := range rows {
				if err := ctx.Err(); err != nil {
					return err
				}
				activityDate := utcFunnelDay(row.ReceivedAt)
				seed := FunnelActivityDay{
					Environment: row.Environment, UserID: row.UserID, ActivityDate: activityDate,
					FirstSeenAt: row.ReceivedAt, LastSeenAt: row.ReceivedAt,
				}
				created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed)
				if created.Error != nil {
					return created.Error
				}
				batchReconciled += created.RowsAffected
				if err := upsertFunnelActivityDay(tx, row.Environment, row.UserID, row.ReceivedAt); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return reconciled, err
		}
		reconciled += batchReconciled
		lastID = rows[len(rows)-1].ID
	}
}

func deleteFunnelEventsBefore(ctx context.Context, cutoff int64, batchSize int) (int64, error) {
	return deleteFunnelRowsByID(ctx, &FunnelEvent{}, batchSize, func(db *gorm.DB) *gorm.DB {
		return db.Where("received_at < ?", cutoff)
	})
}

func anonymizeInactiveFunnelVisitors(ctx context.Context, cutoff int64, batchSize int) (int64, error) {
	var updated int64
	for {
		ids, err := loadFunnelRowIDs(ctx, &FunnelVisitor{}, batchSize, func(db *gorm.DB) *gorm.DB {
			return db.Where("last_seen_at < ? AND visitor_hmac IS NOT NULL", cutoff)
		})
		if err != nil || len(ids) == 0 {
			return updated, err
		}
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		write := DB.WithContext(ctx).Model(&FunnelVisitor{}).Where("id IN ?", ids).Update("visitor_hmac", nil)
		if write.Error != nil {
			return updated, write.Error
		}
		updated += write.RowsAffected
	}
}

func deleteInactiveUnlinkedFunnelVisitors(ctx context.Context, rawCutoff int64, batchSize int) (int64, error) {
	return deleteFunnelRowsByID(ctx, &FunnelVisitor{}, batchSize, func(db *gorm.DB) *gorm.DB {
		return db.Where("last_seen_at < ? AND visitor_hmac IS NULL AND identity_state IN ?", rawCutoff, []string{FunnelIdentityAnonymous, FunnelIdentityAmbiguous}).
			Where("NOT EXISTS (SELECT 1 FROM funnel_events WHERE funnel_events.visitor_id = funnel_visitors.id)")
	})
}

func deleteExpiredLinkedFunnelVisitors(ctx context.Context, activityCutoff int64, batchSize int) (int64, error) {
	return deleteFunnelRowsByID(ctx, &FunnelVisitor{}, batchSize, func(db *gorm.DB) *gorm.DB {
		return db.Where("visitor_hmac IS NULL AND identity_state = ? AND (first_slp_at = 0 OR first_slp_at < ?)", FunnelIdentityLinked, activityCutoff).
			Where("NOT EXISTS (SELECT 1 FROM funnel_events WHERE funnel_events.visitor_id = funnel_visitors.id)")
	})
}

func deleteFunnelActivityDaysBefore(ctx context.Context, cutoff int64, batchSize int) (int64, error) {
	return deleteFunnelRowsByID(ctx, &FunnelActivityDay{}, batchSize, func(db *gorm.DB) *gorm.DB {
		return db.Where("activity_date < ?", cutoff)
	})
}

func deleteFunnelRowsByID(ctx context.Context, value any, batchSize int, scope func(*gorm.DB) *gorm.DB) (int64, error) {
	var deleted int64
	for {
		ids, err := loadFunnelRowIDs(ctx, value, batchSize, scope)
		if err != nil || len(ids) == 0 {
			return deleted, err
		}
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		write := DB.WithContext(ctx).Where("id IN ?", ids).Delete(value)
		if write.Error != nil {
			return deleted, write.Error
		}
		deleted += write.RowsAffected
	}
}

func loadFunnelRowIDs(ctx context.Context, value any, batchSize int, scope func(*gorm.DB) *gorm.DB) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, batchSize)
	query := DB.WithContext(ctx).Model(value).Select("id")
	query = scope(query)
	err := query.Order("id ASC").Limit(batchSize).Scan(&ids).Error
	return ids, err
}

func utcFunnelDay(value int64) int64 {
	return value - value%86400
}
