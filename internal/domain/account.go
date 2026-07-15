package domain

import (
	"errors"
	"time"
)

const (
	StatusActive = "active"
	StatusClosed = "closed"
	StatusFrozen = "frozen"
)

var ErrAccountNotFound = errors.New("account not found")

type Account struct {
	ID        string
	Status    string
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
