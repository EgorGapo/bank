package domain

import (
	"errors"
	"time"
)

const (
	StatusActive    = "active"
	StatusClosed    = "closed"
	StatusFrozen    = "frozen"
	TypeDeposit     = "deposit"
	TypeWithdraw    = "withdraw"
	TypeTransfer    = "transfer"
	StatusCompleted = "completed"
	StatusFailed    = "failed"

	ErrCodeInsufficientFunds = "insufficient_funds"
)

var ErrAccountNotFound = errors.New("account not found")
var ErrIdempotencyKeyReuse = errors.New("reuse of idempotency key")
var ErrNotEnoughMoney = errors.New("not enough money on the account")

type Account struct {
	ID        string
	Status    string
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Transfer struct {
	ID             string
	IdempotencyKey string
	FromAccountID  *string
	ToAccountID    *string
	Type           string
	Amount         int64
	Status         string
	ErrCode        string
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

type LedgerEntry struct {
	ID           int64
	TransferID   string
	AccountID    string
	Amount       int64
	BalanceAfter int64
	CreatedAt    time.Time
}

type HistoryPage struct {
	Entries    []LedgerEntry
	NextCursor int64
	HasMore    bool
}
