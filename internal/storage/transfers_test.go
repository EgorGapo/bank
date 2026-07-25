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
func newTestStorage(t *testing.T) *postgres {
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
func createTestAccount(t *testing.T, s *postgres) *domain.Account {
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
func accountBalance(t *testing.T, s *postgres, accountID string) int64 {
	t.Helper()
	var balance int64
	err := s.db.QueryRow(context.Background(),
		`SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&balance)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return balance
}

func transferStatus(t *testing.T, s *postgres, ID string) string {
	t.Helper()
	var status string
	err := s.db.QueryRow(context.Background(),
		`SELECT status FROM transfers WHERE id = $1`, ID).Scan(&status)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	return status
}

// transferErrorCode — error_code операции (NULL → "").
func transferErrorCode(t *testing.T, s *postgres, ID string) string {
	t.Helper()
	var code *string
	err := s.db.QueryRow(context.Background(),
		`SELECT error_code FROM transfers WHERE id = $1`, ID).Scan(&code)
	if err != nil {
		t.Fatalf("read error_code: %v", err)
	}
	if code == nil {
		return ""
	}
	return *code
}

// ledgerSumByAccount — сумма проводок счёта; должна совпадать с его балансом.
func ledgerSumByAccount(t *testing.T, s *postgres, accountID string) int64 {
	t.Helper()
	var sum int64
	err := s.db.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = $1`, accountID).Scan(&sum)
	if err != nil {
		t.Fatalf("read ledger sum: %v", err)
	}
	return sum
}

// ledgerSumByTransfer — сумма проводок одной операции; для transfer должна быть 0 (double-entry).
func ledgerSumByTransfer(t *testing.T, s *postgres, transferID string) (count int, sum int64) {
	t.Helper()
	err := s.db.QueryRow(context.Background(),
		`SELECT count(*), COALESCE(SUM(amount), 0) FROM ledger_entries WHERE transfer_id = $1`,
		transferID).Scan(&count, &sum)
	if err != nil {
		t.Fatalf("read ledger by transfer: %v", err)
	}
	return count, sum
}

// ledgerEntryAmount — сумма проводки конкретного счёта в рамках операции.
func ledgerEntryAmount(t *testing.T, s *postgres, transferID, accountID string) (amount, balanceAfter int64) {
	t.Helper()
	err := s.db.QueryRow(context.Background(),
		`SELECT amount, balance_after FROM ledger_entries WHERE transfer_id = $1 AND account_id = $2`,
		transferID, accountID).Scan(&amount, &balanceAfter)
	if err != nil {
		t.Fatalf("read ledger entry: %v", err)
	}
	return amount, balanceAfter
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

		second, err := s.Deposit(ctx, 300, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("retry deposit: %v", err)
		}

		if second.ID != first.ID {
			t.Errorf("retry returned different transfer: got %s, want %s", second.ID, first.ID)
		}

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

func TestWithdraw(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		acc := createTestAccount(t, s)
		depositID := uuid.NewString()
		keyDeposit := uuid.NewString()
		_, err := s.Deposit(ctx, 500, depositID, acc.ID, keyDeposit)
		if err != nil {
			t.Fatalf("deposit: %v", err)
		}
		withdrawID := uuid.NewString()
		keyWithdraw := uuid.NewString()
		trWithdraw, err := s.Withdraw(ctx, 200, withdrawID, acc.ID, keyWithdraw)
		if err != nil {
			t.Fatalf("withdraw: %v", err)
		}

		if trWithdraw.Status != domain.StatusCompleted {
			t.Error("Invalid status")
		}
		if trWithdraw.ID != withdrawID {
			t.Errorf("invalid id, have %v, want %v", trWithdraw.ID, withdrawID)
		}

		if trWithdraw.CompletedAt == nil {
			t.Error("completed_at is nil, want set")
		}
		if got := accountBalance(t, s, acc.ID); got != 300 {
			t.Errorf("account balance: got %d, want 300", got)
		}

		var entries int
		var ledgerAmount, balanceAfter int64
		err = s.db.QueryRow(ctx,
			`SELECT count(*), max(amount), max(balance_after) FROM ledger_entries WHERE transfer_id = $1`,
			withdrawID).Scan(&entries, &ledgerAmount, &balanceAfter)
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		if entries != 1 {
			t.Errorf("ledger entries: got %d, want 1", entries)
		}
		if ledgerAmount != -200 {
			t.Errorf("ledger amount: got %d, want -200", ledgerAmount)
		}
		if balanceAfter != 300 {
			t.Errorf("balance_after: got %d, want 300", balanceAfter)
		}
	})

	t.Run("retry with same key returns same transfer", func(t *testing.T) {
		acc := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 500, uuid.NewString(), acc.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		key := uuid.NewString()

		first, err := s.Withdraw(ctx, 200, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("first withdraw: %v", err)
		}
		second, err := s.Withdraw(ctx, 200, uuid.NewString(), acc.ID, key)
		if err != nil {
			t.Fatalf("retry withdraw: %v", err)
		}

		if second.ID != first.ID {
			t.Errorf("retry returned different transfer: got %s, want %s", second.ID, first.ID)
		}

		if got := accountBalance(t, s, acc.ID); got != 300 {
			t.Errorf("balance after retry: got %d, want 300", got)
		}
	})

	t.Run("reuse deposit key in withdraw returns error", func(t *testing.T) {
		acc := createTestAccount(t, s)
		key := uuid.NewString()
		if _, err := s.Deposit(ctx, 300, uuid.NewString(), acc.ID, key); err != nil {
			t.Fatalf("deposit: %v", err)
		}

		_, err := s.Withdraw(ctx, 300, uuid.NewString(), acc.ID, key)
		if !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
			t.Errorf("want ErrIdempotencyKeyReuse, got %v", err)
		}
		if got := accountBalance(t, s, acc.ID); got != 300 {
			t.Errorf("balance: got %d, want 300", got)
		}
	})

	t.Run("1000 thieves", func(t *testing.T) {
		acc := createTestAccount(t, s)
		depositID := uuid.NewString()
		keyDeposit := uuid.NewString()
		_, err := s.Deposit(ctx, 500, depositID, acc.ID, keyDeposit)
		if err != nil {
			t.Fatalf("deposit: %v", err)
		}
		wg := &sync.WaitGroup{}
		errChan := make(chan error, 1000)
		transfers := make(chan string, 1000)
		for range 1000 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				withdrawID := uuid.NewString()
				keyWithdraw := uuid.NewString()
				_, err := s.Withdraw(ctx, 1, withdrawID, acc.ID, keyWithdraw)
				if err != nil {
					errChan <- err
					if errors.Is(err, domain.ErrNotEnoughMoney) {
						transfers <- withdrawID
					}
				}
			}()
		}
		wg.Wait()
		close(errChan)
		close(transfers)
		counter := 0
		for err := range errChan {
			counter++
			if !errors.Is(err, domain.ErrNotEnoughMoney) {
				t.Errorf("should be ErrNotEnoughMoney error, got %v: ", err)
			}
		}
		if counter != 500 {
			t.Errorf("want 500, got %v", counter)
		}
		counter = 0
		for tr := range transfers {
			counter++
			if got := transferStatus(t, s, tr); got != domain.StatusFailed {
				t.Errorf("should be StatusFailed, got %v: ", got)
			}
		}
		if counter != 500 {
			t.Errorf("want 500, got %v", counter)
		}

		if got := accountBalance(t, s, acc.ID); got != 0 {
			t.Errorf("account balance: got %d, want 0", got)
		}

		var ledgerSum int64
		err = s.db.QueryRow(ctx,
			`SELECT COALESCE(SUM(amount), -1) FROM ledger_entries WHERE account_id = $1`,
			acc.ID).Scan(&ledgerSum)
		if err != nil {
			t.Fatalf("read ledger sum: %v", err)
		}
		if ledgerSum != 0 {
			t.Errorf("ledger sum: got %d, want 0 (+500 deposit, -500 withdrawals)", ledgerSum)
		}
	})
}

func TestTransfer(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 1000, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		transferID := uuid.NewString()
		tr, err := s.Transfer(ctx, 1000, transferID, acc1.ID, acc2.ID, uuid.NewString())
		if err != nil {
			t.Fatalf("transfer: %v", err)
		}

		if tr.Status != domain.StatusCompleted {
			t.Errorf("status: got %s, want %s", tr.Status, domain.StatusCompleted)
		}
		if tr.Type != domain.TypeTransfer {
			t.Errorf("type: got %s, want %s", tr.Type, domain.TypeTransfer)
		}
		if tr.CompletedAt == nil {
			t.Error("completed_at is nil, want set")
		}

		balance1 := accountBalance(t, s, acc1.ID)
		balance2 := accountBalance(t, s, acc2.ID)
		if balance1 != 0 {
			t.Errorf("want 0, got %v", balance1)
		}
		if balance2 != 1000 {
			t.Errorf("want 1000, got %v", balance2)
		}
		count, sum := ledgerSumByTransfer(t, s, transferID)
		if count != 2 {
			t.Errorf("ledger entries: got %d, want 2", count)
		}
		if sum != 0 {
			t.Errorf("ledger sum for transfer: got %d, want 0", sum)
		}

		// Знаки и balance_after у каждой стороны.
		fromAmount, fromBalanceAfter := ledgerEntryAmount(t, s, transferID, acc1.ID)
		if fromAmount != -1000 {
			t.Errorf("from entry amount: got %d, want -1000", fromAmount)
		}
		if fromBalanceAfter != 0 {
			t.Errorf("from balance_after: got %d, want 0", fromBalanceAfter)
		}
		toAmount, toBalanceAfter := ledgerEntryAmount(t, s, transferID, acc2.ID)
		if toAmount != 1000 {
			t.Errorf("to entry amount: got %d, want 1000", toAmount)
		}
		if toBalanceAfter != 1000 {
			t.Errorf("to balance_after: got %d, want 1000", toBalanceAfter)
		}
	})

	t.Run("not enough money", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 1000, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		transferID := uuid.NewString()
		_, err := s.Transfer(ctx, 1300, transferID, acc1.ID, acc2.ID, uuid.NewString())
		if err == nil {
			t.Fatalf("no err")
		}
		if !errors.Is(err, domain.ErrNotEnoughMoney) {
			t.Errorf("want domain.ErrNotEnoughMoney, got %v", err)
		}

		// Деньги НЕ двигались.
		if got := accountBalance(t, s, acc1.ID); got != 1000 {
			t.Errorf("from balance: got %d, want 1000 (unchanged)", got)
		}
		if got := accountBalance(t, s, acc2.ID); got != 0 {
			t.Errorf("to balance: got %d, want 0 (unchanged)", got)
		}

		if got := transferStatus(t, s, transferID); got != domain.StatusFailed {
			t.Errorf("status: got %s, want %s", got, domain.StatusFailed)
		}
		if got := transferErrorCode(t, s, transferID); got != domain.ErrCodeInsufficientFunds {
			t.Errorf("error_code: got %q, want %q", got, domain.ErrCodeInsufficientFunds)
		}
		if count, _ := ledgerSumByTransfer(t, s, transferID); count != 0 {
			t.Errorf("ledger entries: got %d, want 0 (nothing posted)", count)
		}
	})

	t.Run("retry with same key returns same transfer", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 500, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		key := uuid.NewString()

		first, err := s.Transfer(ctx, 200, uuid.NewString(), acc1.ID, acc2.ID, key)
		if err != nil {
			t.Fatalf("first transfer: %v", err)
		}
		second, err := s.Transfer(ctx, 200, uuid.NewString(), acc1.ID, acc2.ID, key)
		if err != nil {
			t.Fatalf("retry transfer: %v", err)
		}

		if second.ID != first.ID {
			t.Errorf("retry returned different transfer: got %s, want %s", second.ID, first.ID)
		}
		// Деньги не ушли дважды.
		if got := accountBalance(t, s, acc1.ID); got != 300 {
			t.Errorf("from balance after retry: got %d, want 300", got)
		}
		if got := accountBalance(t, s, acc2.ID); got != 200 {
			t.Errorf("to balance after retry: got %d, want 200", got)
		}
	})

	t.Run("reuse key with different params returns error", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 500, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		key := uuid.NewString()

		if _, err := s.Transfer(ctx, 200, uuid.NewString(), acc1.ID, acc2.ID, key); err != nil {
			t.Fatalf("first transfer: %v", err)
		}
		// Тот же ключ, другая сумма.
		_, err := s.Transfer(ctx, 300, uuid.NewString(), acc1.ID, acc2.ID, key)
		if !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
			t.Errorf("want ErrIdempotencyKeyReuse, got %v", err)
		}
		if got := accountBalance(t, s, acc1.ID); got != 300 {
			t.Errorf("from balance: got %d, want 300 (unchanged)", got)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		acc := createTestAccount(t, s)
		missing := uuid.NewString()

		_, err := s.Transfer(ctx, 100, uuid.NewString(), acc.ID, missing, uuid.NewString())
		if !errors.Is(err, domain.ErrAccountNotFound) {
			t.Errorf("unknown to-account: want ErrAccountNotFound, got %v", err)
		}

		_, err = s.Transfer(ctx, 100, uuid.NewString(), missing, acc.ID, uuid.NewString())
		if !errors.Is(err, domain.ErrAccountNotFound) {
			t.Errorf("unknown from-account: want ErrAccountNotFound, got %v", err)
		}
	})

	t.Run("1000 free dollars", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 1000, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		// Все переводы обязаны пройти: ошибку репортит Wait, а не каждая горутина.
		g, gctx := errgroup.WithContext(ctx)
		for range 1000 {
			g.Go(func() error {
				_, err := s.Transfer(gctx, 1, uuid.NewString(), acc1.ID, acc2.ID, uuid.NewString())
				return err
			})
		}
		if err := g.Wait(); err != nil {
			t.Fatalf("transfer: %v", err)
		}
		balance1 := accountBalance(t, s, acc1.ID)
		balance2 := accountBalance(t, s, acc2.ID)
		if balance1 != 0 {
			t.Errorf("want 0, got %v", balance1)
		}
		if balance2 != 1000 {
			t.Errorf("want 1000, got %v", balance2)
		}
		// Балансы сходятся с проводками.
		if got := ledgerSumByAccount(t, s, acc1.ID); got != balance1 {
			t.Errorf("ledger sum acc1: got %d, want %d (= balance)", got, balance1)
		}
		if got := ledgerSumByAccount(t, s, acc2.ID); got != balance2 {
			t.Errorf("ledger sum acc2: got %d, want %d (= balance)", got, balance2)
		}
	})

	t.Run("1000 free dollars from man with no money", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 100, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		// Ошибки ожидаемы -> голая Group (без отмены) + считаем исходы.
		errCh := make(chan error, 1000)
		var g errgroup.Group
		for range 1000 {
			g.Go(func() error {
				if _, err := s.Transfer(ctx, 10, uuid.NewString(), acc1.ID, acc2.ID, uuid.NewString()); err != nil {
					errCh <- err
				}
				return nil
			})
		}
		_ = g.Wait()
		close(errCh)

		failed := 0
		for err := range errCh {
			failed++
			if !errors.Is(err, domain.ErrNotEnoughMoney) {
				t.Errorf("want domain.ErrNotEnoughMoney, got %v", err)
			}
		}
		// Денег хватило ровно на 10 переводов по 10 -> 990 отказов.
		if failed != 990 {
			t.Errorf("failed transfers: got %d, want 990", failed)
		}

		balance1 := accountBalance(t, s, acc1.ID)
		balance2 := accountBalance(t, s, acc2.ID)
		if balance1 != 0 {
			t.Errorf("want 0, got %v", balance1)
		}
		if balance2 != 100 {
			t.Errorf("want 100, got %v", balance2)
		}
		if got := ledgerSumByAccount(t, s, acc1.ID); got != balance1 {
			t.Errorf("ledger sum acc1: got %d, want %d (= balance)", got, balance1)
		}
	})

	t.Run("friends 200", func(t *testing.T) {
		acc1 := createTestAccount(t, s)
		acc2 := createTestAccount(t, s)
		if _, err := s.Deposit(ctx, 200, uuid.NewString(), acc1.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		if _, err := s.Deposit(ctx, 200, uuid.NewString(), acc2.ID, uuid.NewString()); err != nil {
			t.Fatalf("deposit: %v", err)
		}
		var g errgroup.Group
		for range 200 {
			g.Go(func() error {
				_, err := s.Transfer(ctx, 1, uuid.NewString(), acc1.ID, acc2.ID, uuid.NewString())
				return err
			})
			g.Go(func() error {
				_, err := s.Transfer(ctx, 1, uuid.NewString(), acc2.ID, acc1.ID, uuid.NewString())
				return err
			})
		}
		if err := g.Wait(); err != nil {
			t.Errorf("got %v", err)
		}
		balance1 := accountBalance(t, s, acc1.ID)
		balance2 := accountBalance(t, s, acc2.ID)
		if balance1 != 200 {
			t.Errorf("want 200, got %v", balance1)
		}
		if balance2 != 200 {
			t.Errorf("want 200, got %v", balance2)
		}
		if got := ledgerSumByAccount(t, s, acc1.ID); got != balance1 {
			t.Errorf("ledger sum acc1: got %d, want %d (= balance)", got, balance1)
		}
		if got := ledgerSumByAccount(t, s, acc2.ID); got != balance2 {
			t.Errorf("ledger sum acc2: got %d, want %d (= balance)", got, balance2)
		}
	})

}
