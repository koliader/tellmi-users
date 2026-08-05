package dispatcher

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	tellmiotel "github.com/koliader/tellmi-sdk/otel"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

const (
	DefaultPollInterval    = 100 * time.Millisecond
	DefaultBatchSize       = 10
	DefaultCleanupInterval = 5 * time.Minute
	DefaultRetention       = 24 * time.Hour
	DefaultBackoffInitial  = 500 * time.Millisecond
	DefaultBackoffMax      = 10 * time.Second
)

type Config struct {
	PollInterval    time.Duration
	BatchSize       int32
	CleanupInterval time.Duration
	Retention       time.Duration
	BackoffInitial  time.Duration
	BackoffMax      time.Duration
	// MeterProvider supplies the metric meter for the outbox gauge. When nil,
	// a noop meter is used so the dispatcher stays testable without OTel.
	MeterProvider metric.MeterProvider
}

type Dispatcher struct {
	store       db.Store
	sender      rabbitmq.MessageSender
	cfg         Config
	meter       metric.Meter
	outboxGauge metric.Int64ObservableGauge
}

func New(store db.Store, sender rabbitmq.MessageSender, cfg Config) *Dispatcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCleanupInterval
	}
	if cfg.Retention <= 0 {
		cfg.Retention = DefaultRetention
	}
	if cfg.BackoffInitial <= 0 {
		cfg.BackoffInitial = DefaultBackoffInitial
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = DefaultBackoffMax
	}
	meter := noop.NewMeterProvider().Meter("tellmi/outbox")
	if cfg.MeterProvider != nil {
		meter = cfg.MeterProvider.Meter("tellmi/outbox")
	}
	gauge, err := meter.Int64ObservableGauge(
		"outbox_events_unpublished",
		metric.WithDescription("Number of outbox events not yet published"),
	)
	if err != nil {
		log.Warn().Err(err).Msg("outbox dispatcher: cannot create outbox gauge")
		gauge, _ = noop.NewMeterProvider().Meter("tellmi/outbox").Int64ObservableGauge("outbox_events_unpublished")
	}
	return &Dispatcher{store: store, sender: sender, cfg: cfg, meter: meter, outboxGauge: gauge}
}

func (d *Dispatcher) Start(ctx context.Context) {
	go d.dispatchLoop(ctx)
	if d.cfg.Retention > 0 {
		go d.cleanupLoop(ctx)
	}
	if _, err := d.meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		count, err := d.store.CountUnpublishedOutboxEvents(ctx)
		if err != nil {
			return err
		}
		o.ObserveInt64(d.outboxGauge, count)
		return nil
	}, d.outboxGauge); err != nil {
		log.Warn().Err(err).Msg("outbox dispatcher: cannot register outbox gauge callback")
	}
	log.Info().Msg("outbox dispatcher started")
}

func (d *Dispatcher) dispatchLoop(ctx context.Context) {
	backoff := d.cfg.BackoffInitial
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("outbox dispatcher stopped")
			return
		default:
		}

		published, hadFailure := d.dispatchOnce(ctx)
		if hadFailure {
			// batch had a failure: back off exponentially before polling again
			log.Warn().Int("published", published).Dur("backoff", backoff).
				Msg("outbox dispatcher: publish failed, backing off")
			select {
			case <-ctx.Done():
				log.Info().Msg("outbox dispatcher stopped")
				return
			case <-time.After(backoff):
			}
			log.Info().Int("published", published).Dur("backoff", backoff).
				Msg("outbox dispatcher: backoff elapsed, retrying dispatch")
			if backoff < d.cfg.BackoffMax {
				backoff *= 2
				if backoff > d.cfg.BackoffMax {
					backoff = d.cfg.BackoffMax
				}
			}
			continue
		}

		backoff = d.cfg.BackoffInitial
		select {
		case <-ctx.Done():
			log.Info().Msg("outbox dispatcher stopped")
			return
		case <-time.After(d.cfg.PollInterval):
		}
	}
}

// dispatchOnce claims a batch of unpublished events with FOR UPDATE SKIP LOCKED
// inside a single transaction, so concurrent dispatchers never claim the same
// row. Each event is published and then marked published within the same tx.
// A single failing event (poison row) is left unpublished while the rest of the
// batch continues; the tx still commits the successful marks. Returns the number
// of events published and whether the batch had any failure so the loop can back off.
func (d *Dispatcher) dispatchOnce(ctx context.Context) (int, bool) {
	hadFailure := false
	published := 0

	err := d.store.ExecTx(ctx, func(q *db.Queries) error {
		events, err := q.ListUnpublishedOutboxEvents(ctx, d.cfg.BatchSize)
		if err != nil {
			return err
		}

		if len(events) > 0 {
			log.Info().Int("found", len(events)).Msg("outbox dispatcher: found unpublished events")
		}

		for _, event := range events {
			queue := queueForEvent(event.EventType)
			if queue == "" {
				log.Error().Str("event_id", event.ID.String()).Str("event_type", event.EventType).
					Msg("outbox dispatcher: unknown event type")
				continue
			}

			// resume the trace that created this event (stored as traceparent at
			// insert time) so the async publish continues the original span tree
			msgCtx := tellmiotel.ExtractTraceContext(ctx, stringOrEmpty(event.TraceContext))
			_, span := otel.Tracer("tellmi/outbox").Start(msgCtx, "outbox.publish "+event.EventType)
			sendErr := d.sender.SendMessage(msgCtx, queue, event.Payload)
			span.End()

			if sendErr != nil {
				hadFailure = true
				log.Error().Ctx(msgCtx).Err(sendErr).Str("event_id", event.ID.String()).Str("queue", queue).
					Msg("outbox dispatcher: failed to publish event, will retry")
				continue
			}
			published++

			log.Info().Ctx(msgCtx).Str("event_id", event.ID.String()).Str("queue", queue).
				Msg("outbox dispatcher: published event")

			if err := q.MarkOutboxEventPublished(ctx, event.ID); err != nil {
				hadFailure = true
				log.Error().Ctx(msgCtx).Err(err).Str("event_id", event.ID.String()).
					Msg("outbox dispatcher: failed to mark event as published")
				continue
			}

			log.Info().Ctx(msgCtx).Str("event_id", event.ID.String()).Str("queue", queue).
				Msg("outbox dispatcher: marked event as published")
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("outbox dispatcher: failed to claim unpublished events")
		return 0, false
	}

	return published, hadFailure
}

func (d *Dispatcher) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			interval := pgtype.Interval{
				Microseconds: d.cfg.Retention.Microseconds(),
				Valid:        true,
			}
			if err := d.store.DeletePublishedOutboxEvents(ctx, interval); err != nil {
				log.Error().Err(err).Msg("outbox dispatcher: failed to delete old published events")
			}
		}
	}
}

func queueForEvent(eventType string) string {
	switch eventType {
	case "userCreated":
		return rabbitmq.UserCreatedQueue
	case "userUpdated":
		return rabbitmq.UserUpdatedQueue
	default:
		return ""
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
