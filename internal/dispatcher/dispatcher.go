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
)

type Config struct {
	PollInterval    time.Duration
	BatchSize       int32
	CleanupInterval time.Duration
	Retention       time.Duration
}

type Dispatcher struct {
	store   db.Store
	sender  rabbitmq.MessageSender
	cfg     Config
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
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("outbox dispatcher stopped")
			return
		case <-ticker.C:
			d.dispatchOnce(ctx)
		}
	}
}

func (d *Dispatcher) dispatchOnce(ctx context.Context) {
	events, err := d.store.ListUnpublishedOutboxEvents(ctx, d.cfg.BatchSize)
	if err != nil {
		log.Error().Err(err).Msg("outbox dispatcher: failed to list unpublished events")
		return
	}

	for _, event := range events {
		queue := queueForEvent(event.EventType)
		if queue == "" {
			log.Error().Str("event_id", event.ID.String()).Str("event_type", event.EventType).
				Msg("outbox dispatcher: unknown event type")
			continue
		}

		if err := d.sender.SendMessage(ctx, queue, event.Payload); err != nil {
			log.Error().Err(err).Str("event_id", event.ID.String()).Str("queue", queue).
				Msg("outbox dispatcher: failed to publish event")
			// broker is unavailable or rejected the batch; stop rather than racing
			// confirms for the remaining events
			break
		}

		if err := d.store.MarkOutboxEventPublished(ctx, event.ID); err != nil {
			log.Error().Err(err).Str("event_id", event.ID.String()).
				Msg("outbox dispatcher: failed to mark event as published")
		}
	}
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
