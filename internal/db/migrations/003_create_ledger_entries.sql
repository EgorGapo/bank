
-- +goose Up
CREATE TABLE ledger_entries(
    id BIGSERIAL PRIMARY KEY,
    transfer_id UUID NOT NULL,
    account_id UUID NOT NULL,
    amount BIGINT NOT NULL CONSTRAINT ledger_entries_amount_check CHECK (amount <> 0),
    balance_after BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT ledger_entries_fk_transfer_id FOREIGN KEY (transfer_id) REFERENCES transfers(id),
    CONSTRAINT ledger_entries_fk_account_id FOREIGN KEY (account_id ) REFERENCES accounts(id)
);
-- +goose Down
DROP TABLE ledger_entries;