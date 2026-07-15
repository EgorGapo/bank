package domain

import "time"

const (
	StatusActive = "active"
	StatuClosed  = "closed"
	StatusFrozen = "frozen"
)

type Account struct {
	ID        string
	Status    string
	Balance   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
