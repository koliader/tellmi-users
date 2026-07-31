-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
  aggregate_type,
  aggregate_id,
  event_type,
  payload
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: ListUnpublishedOutboxEvents :many
SELECT *
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at
LIMIT $1;

-- name: MarkOutboxEventPublished :exec
UPDATE outbox_events
SET published_at = now()
WHERE id = $1;

-- name: DeletePublishedOutboxEvents :exec
DELETE FROM outbox_events
WHERE published_at IS NOT NULL
  AND created_at < now() - $1::interval;
