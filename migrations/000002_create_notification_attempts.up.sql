CREATE TABLE notification_attempts (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id   UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    attempt_number    INT         NOT NULL,
    status            VARCHAR(20) NOT NULL,
    error_message     TEXT,
    provider_response JSONB,
    attempted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_attempts_notification_id ON notification_attempts(notification_id);
