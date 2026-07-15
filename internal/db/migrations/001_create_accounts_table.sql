-- +goose Up


CREATE TABLE accounts 
(
    id UUID PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'active' CONSTRAINT accounts_status_check CHECK (status IN ('active', 'frozen', 'closed')),
    balance BIGINT NOT NULL DEFAULT 0 CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0),
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL 
); 

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_accounts_timestamp() RETURNS TRIGGER AS
$$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE OR REPLACE TRIGGER trigger_update_accounts_timestamp
    BEFORE UPDATE
    ON accounts
    FOR EACH ROW
EXECUTE FUNCTION update_accounts_timestamp();

-- +goose Down
DROP TABLE accounts;
DROP FUNCTION update_accounts_timestamp();