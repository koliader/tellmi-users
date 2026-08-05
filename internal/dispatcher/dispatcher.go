package dispatcher

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/rs/zerolog/log"
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
}

type Dispatcher struct {
	store  db.Store
	sender rabbitmq.MessageSender
	cfg    Config
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
	return &Dispatcher{store: store, sender: sender, cfg: cfg}
}

func (d *Dispatcher) Start(ctx context.Context) {
	go d.dispatchLoop(ctx)
	if d.cfg.Retention > 0 {
		go d.cleanupLoop(ctx)
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

		if !d.dispatchOnce(ctx) {
			// batch had a failure: back off exponentially before polling again
			log.Info().Dur("backoff", backoff).Msg("outbox dispatcher backing off")
			select {
			case <-ctx.Done():
				log.Info().Msg("outbox dispatcher stopped")
				return
			case <-time.After(backoff):
			}
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
// batch continues; the tx still commits the successful marks. Returns false if
// the batch had any failure so the loop can back off.
func (d *Dispatcher) dispatchOnce(ctx context.Context) bool {
	hadFailure := false

	err := d.store.ExecTx(ctx, func(q *db.Queries) error {
		events, err := q.ListUnpublishedOutboxEvents(ctx, d.cfg.BatchSize)
		if err != nil {
			return err
		}

		for _, event := range events {
			queue := queueForEvent(event.EventType)
			if queue == "" {
				log.Error().Str("event_id", event.ID.String()).Str("event_type", event.EventType).
					Msg("outbox dispatcher: unknown event type")
				continue
			}

			if err := d.sender.SendMessage(ctx, queue, event.Payload); err != nil {
				hadFailure = true
				log.Error().Err(err).Str("event_id", event.ID.String()).Str("queue", queue).
					Msg("outbox dispatcher: failed to publish event, will retry")
				continue
			}

			if err := q.MarkOutboxEventPublished(ctx, event.ID); err != nil {
				hadFailure = true
				log.Error().Err(err).Str("event_id", event.ID.String()).
					Msg("outbox dispatcher: failed to mark event as published")
			}
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("outbox dispatcher: failed to claim unpublished events")
		return false
	}

	return !hadFailure
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
