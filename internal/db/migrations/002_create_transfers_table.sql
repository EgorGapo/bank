-- +goose Up

CREATE TABLE transfers
(
    id UUID PRIMARY KEY,
    idempotency_key UUID UNIQUE NOT NULL, 
    type TEXT NOT NULL CONSTRAINT transfers_type_check CHECK (type IN ('deposit', 'withdraw', 'transfer')),
    from_account_id UUID,
    to_account_id UUID,
    amount BIGINT NOT NULL CONSTRAINT transfers_amount_check CHECK (amount>0),
    status TEXT NOT NULL DEFAULT 'pending' CONSTRAINT transfers_status_check CHECK (status IN ('pending', 'completed', 'failed')),
    error_code TEXT,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    completed_at TIMESTAMPTZ,
    CONSTRAINT transfers_fk_from_account_id FOREIGN KEY (from_account_id) REFERENCES accounts(id),
    CONSTRAINT transfers_fk_to_account_id FOREIGN KEY (to_account_id) REFERENCES accounts(id),
    CONSTRAINT transfers_type_refs_check CHECK (
    (type = 'deposit'  AND from_account_id IS NULL     AND to_account_id IS NOT NULL) OR
    (type = 'withdraw' AND from_account_id IS NOT NULL AND to_account_id IS NULL)     OR
    (type = 'transfer' AND from_account_id IS NOT NULL AND to_account_id IS NOT NULL
                       AND from_account_id <> to_account_id)
)
);

-- +goose Down
DROP TABLE transfers;