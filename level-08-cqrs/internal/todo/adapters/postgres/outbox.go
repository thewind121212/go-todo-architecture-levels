package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"example.com/todo/level08/internal/todo/outbox"
)

// Outbox implements outbox.Store. The rows are written by TodoRepository.Save.
type Outbox struct {
	db *gorm.DB
}

var _ outbox.Store = (*Outbox)(nil)

func NewOutbox(db *gorm.DB) *Outbox {
	return &Outbox{db: db}
}

// Unprocessed reads the oldest unprocessed rows. With one relay this plain SELECT is enough;
// several relays would add FOR UPDATE SKIP LOCKED inside their transaction, so each row
// goes to one of them (see the README).
func (o *Outbox) Unprocessed(ctx context.Context, limit int) ([]outbox.Message, error) {
	var models []outboxModel
	// UUIDv7 ids are time-ordered, so id ASC means oldest first.
	err := conn(ctx, o.db).Where("processed_at IS NULL").Order("id ASC").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select outbox: %w", err)
	}
	messages := make([]outbox.Message, 0, len(models))
	for i := range models {
		messages = append(messages, toMessage(&models[i]))
	}
	return messages, nil
}

func (o *Outbox) MarkProcessed(ctx context.Context, id string, at time.Time) error {
	err := conn(ctx, o.db).Model(&outboxModel{}).Where("id = ?", id).Update("processed_at", at).Error
	if err != nil {
		return fmt.Errorf("mark outbox processed: %w", err)
	}
	return nil
}
