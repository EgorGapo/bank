package usecases

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/EgorGapo/bank/internal/domain"
)

type fakeStorage struct {
	BankStorage
	historyFn func(ctx context.Context, accountID string, cursor, limit int64) ([]domain.LedgerEntry, error)
}

func (f fakeStorage) GetHistory(ctx context.Context, accountID string, cursor, limit int64) ([]domain.LedgerEntry, error) {
	return f.historyFn(ctx, accountID, cursor, limit)
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func makeEntries(n int, startID int64) []domain.LedgerEntry {
	entries := make([]domain.LedgerEntry, 0, n)
	for i := range n {
		entries = append(entries, domain.LedgerEntry{
			ID:           startID - int64(i),
			TransferID:   "transfer-" + string(rune('a'+i)),
			AccountID:    "acc-1",
			Amount:       int64(100 * (i + 1)),
			BalanceAfter: int64(1000 - 100*i),
			CreatedAt:    time.Now(),
		})
	}
	return entries
}

func TestGetHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("full page: has more, cursor = id последней отданной", func(t *testing.T) {
		var gotLimit int64
		s := &fakeStorage{historyFn: func(_ context.Context, _ string, _, limit int64) ([]domain.LedgerEntry, error) {
			gotLimit = limit
			return makeEntries(5, 105), nil
		}}
		b := NewBank(s, quietLogger())

		page, err := b.GetHistory(ctx, "acc-1", 999, 4)
		if err != nil {
			t.Fatalf("get history: %v", err)
		}

		if gotLimit != 5 {
			t.Errorf("storage limit: got %d, want 5 (limit+1)", gotLimit)
		}
		if len(page.Entries) != 4 {
			t.Errorf("entries: got %d, want 4", len(page.Entries))
		}
		if !page.HasMore {
			t.Error("has_more: got false, want true")
		}
		if page.NextCursor != 102 {
			t.Errorf("next_cursor: got %d, want 102", page.NextCursor)
		}
	})

	t.Run("full page, no more", func(t *testing.T) {

		s := &fakeStorage{historyFn: func(_ context.Context, _ string, _, limit int64) ([]domain.LedgerEntry, error) {
			return makeEntries(4, 105), nil
		}}
		b := NewBank(s, quietLogger())

		page, err := b.GetHistory(ctx, "acc-1", 999, 4)
		if err != nil {
			t.Fatalf("get history: %v", err)
		}
		if len(page.Entries) != 4 {
			t.Errorf("entries: got %d, want 4", len(page.Entries))
		}
		if page.HasMore {
			t.Error("has_more: got true, want false")
		}
		if page.NextCursor != 0 {
			t.Errorf("next_cursor: got %d, want 0", page.NextCursor)
		}
	})

	t.Run("want more than have", func(t *testing.T) {

		s := &fakeStorage{historyFn: func(_ context.Context, _ string, _, limit int64) ([]domain.LedgerEntry, error) {
			return makeEntries(4, 105), nil
		}}
		b := NewBank(s, quietLogger())

		page, err := b.GetHistory(ctx, "acc-1", 999, 10)
		if err != nil {
			t.Fatalf("get history: %v", err)
		}
		if len(page.Entries) != 4 {
			t.Errorf("entries: got %d, want 4", len(page.Entries))
		}
		if page.HasMore {
			t.Error("has_more: got true, want false")
		}
		if page.NextCursor != 0 {
			t.Errorf("next_cursor: got %d, want 0", page.NextCursor)
		}
	})

}
