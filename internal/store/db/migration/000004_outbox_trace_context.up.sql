ALTER TABLE "outbox_events"
  ADD COLUMN "trace_context" text;

CREATE INDEX "outbox_events_trace_context_idx"
  ON "outbox_events" ("trace_context")
  WHERE "published_at" IS NULL;
