package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

// newTestStorage подключается к локальной базе (make up).
// Если базы нет — тест пропускается, а не падает.
func newTestStorage(t *testing.T) *Postgres {
	t.Helper()
	_ = godotenv.Load("../../.env")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("can not get application config: %s", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.Postgres.DSN())
	if err != nil {
		t.Skipf("no test db: %v", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		t.Skipf("no test db: %v", err)
	}

	// Закрыть пул после завершения теста.
	t.Cleanup(pool.Close)

	quietLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewPostgres(pool, quietLogger)
}

// createTestAccount — фикстура: свежий счёт для теста.
func createTestAccount(t *testing.T, s *Postgres) *domain.Account {
	t.Helper()
	acc := &domain.Account{
		ID:     uuid.NewString(),
		Status: domain.StatusActive,
	}
	if err := s.CreateAccount(context.Background(), acc); err != nil {
		t.Fatalf("create fixture account: %v", err)
	}
	return acc
}

// accountBalance — прямой взгляд в базу, мимо тестируемого кода.
func accountBalance(t *testing.T, s *Postgres, accountID string) int64 {
	t.Helper()
	var balance int64
	err := s.db.QueryRow(context.Background(),
		`SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&balance)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return balance
}

func TestDeposit(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		acc := createTestAccount(t, s)
		transferID := uuid.NewString()
		key := uuid.NewString()

		tr, err := s.Deposit(ctx, 500, transferID, acc.ID, key)
		if err != nil {
			t.Fatalf("deposit: %v", err)
		}

		// Проверяем возвращённый transfer.
		if tr.ID != transferID {
			t.Errorf("transfer id: got %s, want %s", tr.ID, transferID)
		}
		if tr.Status != domain.StatusCompleted {
			t.Errorf("status: got %s, want %s", tr.Status, domain.StatusCompleted)
		}
		if tr.Amount != 500 {
			t.Errorf("amount: got %d, want 500", tr.Amount)
		}
		if tr.CompletedAt == nil {
			t.Error("completed_at is nil, want set")
		}

		// Проверяем последствия в базе, мимо тестируемого кода.
		if got := accountBalance(t, s, acc.ID); got != 500 {
			t.Errorf("account balance: got %d, want 500", got)
		}

		var entries int
		var balanceAfter int64
		err = s.db.QueryRow(ctx,
			`SELECT count(*), max(balance_after) FROM ledger_entries WHERE transfer_id = $1`,
			transferID).Scan(&entries, &balanceAfter)
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		if entries != 1 {
			t.Errorf("ledger entries: got %d, want 1", entries)
		}
		if balanceAfter != 500 {
			t.Errorf("balance_after: got %d, want 500", balanceAfter)
		}
	})

	t.Run("retry with same key returns same transfer", func(t *testing.T) {
		acc := createTestAccount(t, s)
		key := uuid.NewString()

		first, err := s.Deposit(ctx, 300, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("first deposit: %v", err)
		}

		// Ретрай: тот же ключ и параметры, новый transferID (как сделал бы usecase).
		second, err := s.Deposit(ctx, 300, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("retry deposit: %v", err)
		}

		if second.ID != first.ID {
			t.Errorf("retry returned different transfer: got %s, want %s", second.ID, first.ID)
		}

		// Главное: деньги не задвоились.
		if got := accountBalance(t, s, acc.ID); got != 300 {
			t.Errorf("balance after retry: got %d, want 300", got)
		}
	})

	t.Run("retry with same key but different body returns error", func(t *testing.T) {
		acc := createTestAccount(t, s)
		key := uuid.NewString()
		_, err := s.Deposit(ctx, 300, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("first deposit: %v", err)
		}
		_, err = s.Deposit(ctx, 600, uuid.NewString(), acc.ID, key)
		if !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
			t.Errorf("should be ErrIdempotencyKeyReuse error, got %v: ", err)
		}

	})

	t.Run("account not found", func(t *testing.T) {
		accID := uuid.NewString()
		key := uuid.NewString()
		_, err := s.Deposit(ctx, 300, uuid.NewString(), accID, key)
		if !errors.Is(err, domain.ErrAccountNotFound) {
			t.Fatalf("want ErrAccountNotFound, got: %v", err)
		}
	})

	t.Run("100 writers", func(t *testing.T) {
		acc := createTestAccount(t, s)
		var g errgroup.Group
		for range 100 {
			g.Go(func() error {
				key := uuid.NewString()
				_, err := s.Deposit(ctx, 1, uuid.NewString(), acc.ID, key)
				return err
			})
		}
		if err := g.Wait(); err != nil {
			t.Fatalf(" got: %v", err)
		}
		if got := accountBalance(t, s, acc.ID); got != 100 {
			t.Errorf("got %d, want 100", got)
		}

	})

	t.Run("100 writers, 99 errors", func(t *testing.T) {
		errCh := make(chan error, 100) // буфер = числу горутин, никто не блокируется
		acc := createTestAccount(t, s)
		key := uuid.NewString()
		wg := &sync.WaitGroup{}
		ansBalance := 0
		done := make(chan struct{})
		counter := 0
		go func() {
			defer close(done)
			for err := range errCh {
				counter++
				if !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
					t.Errorf("should be ErrIdempotencyKeyReuse error, got %v: ", err)
				}
			}
		}()

		for i := 1; i <= 100; i++ {
			wg.Add(1)
			go func(balance int) {
				defer wg.Done()
				_, err := s.Deposit(ctx, int64(balance), uuid.NewString(), acc.ID, key)
				if err != nil {
					errCh <- err
				} else {
					ansBalance = i
				}
			}(i)
		}
		wg.Wait()
		close(errCh)
		<-done
		if counter != 99 {
			t.Fatalf("want 99, got %d", counter)
		}
		if got := accountBalance(t, s, acc.ID); got != int64(ansBalance) {
			t.Errorf("got %d, want %d", got, ansBalance)
		}

	})

}
