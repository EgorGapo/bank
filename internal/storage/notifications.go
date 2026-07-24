package storage

import (
	"context"
	"fmt"
)

const queryInsertNotification = `
	INSERT INTO notifications (event_id, account_id , text)
	VALUES ($1, $2, $3) ON CONFLICT (event_id) DO NOTHING`

func (s *postgres) InsertNotification(ctx context.Context, eventId, accountId, text string) error {
	if _, err := s.db.Exec(ctx, queryInsertNotification, eventId, accountId, text); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}
