CREATE TABLE "outbox_events" (
  "id" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "aggregate_type" varchar NOT NULL,
  "aggregate_id" uuid NOT NULL,
  "event_type" varchar NOT NULL,
  "payload" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "published_at" timestamptz
);

CREATE INDEX "outbox_events_unpublished_idx"
  ON "outbox_events" ("created_at")
  WHERE "published_at" IS NULL;
