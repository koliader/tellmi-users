ALTER TABLE "outbox_events"
  DROP COLUMN IF EXISTS "trace_context";

DROP INDEX IF EXISTS "outbox_events_trace_context_idx";
