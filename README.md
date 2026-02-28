# Notification Service

Event-driven notification system that processes and delivers messages through SMS, Email, and Push channels via async queue workers.

## Stack

- **Go 1.24** — runtime
- **Fiber v2** — HTTP framework
- **Asynq** — Redis-backed task queue
- **PostgreSQL 17** — persistence
- **Redis 7** — queue broker
- **golang-migrate** — database migrations
- **zerolog** — structured logging

## Architecture

```
HTTP Request → Fiber Handler → NotificationService → PostgreSQL (pending)
                                                   → Asynq Queue
                                                         ↓
                                               Worker (Processor)
                                                   → WebhookProvider
                                                   → PostgreSQL (delivered/failed)
```

**Priority queues:** `critical` (6), `default` (3), `low` (1) — weighted round-robin.

**Retry:** exponential backoff `min(2^n × 10s, 1h) + jitter`, max 5 retries.

**Idempotency:** SHA256(recipient|channel|content|idempotency_key) — duplicate requests return 200 instead of 201.

## Setup

### Prerequisites

- Docker & Docker Compose

### Run

```bash
# Copy env and set your webhook.site URL
cp .env.example .env

# Start everything
docker compose up --build
```

| Service | URL |
|---------|-----|
| API | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |
| Asynqmon | http://localhost:8081 |
| Health | http://localhost:8080/health |

### Run locally (without Docker)

```bash
# Requires running postgres and redis
go run ./cmd/server
```

## Tests

```bash
go test ./...
```

## API Examples

### Create notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "user@example.com",
    "channel": "email",
    "content": "Hello!",
    "priority": "high",
    "idempotency_key": "order-123-confirm"
  }'
```

### Schedule a notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Your order is ready.",
    "idempotency_key": "order-123-sms",
    "scheduled_at": "2026-03-01T10:00:00Z"
  }'
```

### Batch create (up to 1000)

```bash
curl -X POST http://localhost:8080/api/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"recipient": "a@example.com", "channel": "email", "content": "Hi", "idempotency_key": "batch-1-a"},
      {"recipient": "b@example.com", "channel": "email", "content": "Hi", "idempotency_key": "batch-1-b"}
    ]
  }'
```

### Get notification

```bash
curl http://localhost:8080/api/v1/notifications/{id}
```

### Get batch status

```bash
curl http://localhost:8080/api/v1/notifications/batch/{batchId}
```

### Cancel notification

```bash
curl -X DELETE http://localhost:8080/api/v1/notifications/{id}
```

### List with filters

```bash
curl "http://localhost:8080/api/v1/notifications?status=failed&channel=email&page=1&limit=20"
```

## Channels & Validation

| Channel | Recipient format | Content limit |
|---------|-----------------|---------------|
| `email` | valid email address | 10 000 chars |
| `sms` | E.164 phone number (e.g. `+905551234567`) | 160 chars |
| `push` | any string | 512 chars |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `REDIS_URL` | — | Redis connection string |
| `WEBHOOK_SITE_URL` | — | External provider URL |
| `SERVER_PORT` | `8080` | HTTP port |
| `WORKER_CONCURRENCY` | `10` | Concurrent workers |
| `MAX_RETRIES` | `5` | Max delivery retries |
| `LOG_LEVEL` | `info` | Log level |
