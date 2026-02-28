CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE notifications (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id            UUID,
    recipient           VARCHAR(255) NOT NULL,
    channel             VARCHAR(50)  NOT NULL,
    content             TEXT         NOT NULL,
    priority            VARCHAR(20)  NOT NULL DEFAULT 'normal',
    status              VARCHAR(20)  NOT NULL DEFAULT 'pending',
    idempotency_key     VARCHAR(255) UNIQUE,
    task_id             VARCHAR(255),
    provider_message_id VARCHAR(255),
    scheduled_at        TIMESTAMPTZ,
    error_message       TEXT,
    retry_count         INT          NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_batch_id   ON notifications(batch_id);
CREATE INDEX idx_notifications_status     ON notifications(status);
CREATE INDEX idx_notifications_channel    ON notifications(channel);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);
CREATE INDEX idx_notifications_scheduled  ON notifications(scheduled_at) WHERE scheduled_at IS NOT NULL;
