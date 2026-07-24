-- +goose Up
CREATE TABLE notifications (
    event_id   UUID PRIMARY KEY,  
    account_id UUID NOT NULL,     
    text       TEXT NOT NULL,  
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE notifications;
