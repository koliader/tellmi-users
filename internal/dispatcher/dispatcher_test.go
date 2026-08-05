package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/koliader/tellmi-sdk/config"
	"github.com/koliader/tellmi-sdk/rabbitmq"
	db "github.com/koliader/tellmi-users/internal/store/db/sqlc"
	"github.com/stretchr/testify/require"
)

type recordingSender struct {
	queues []string
	bodies [][]byte
	err    error
}

func (s *recordingSender) SendMessage(_ context.Context, queue string, body []byte) error {
	if s.err != nil {
		return s.err
	}
	s.queues = append(s.queues, queue)
	s.bodies = append(s.bodies, body)
	return nil
}

type testConfig struct {
	DBSource string `mapstructure:"DB_SOURCE"`
}

var (
	testStore db.Store
	testPool  *pgxpool.Pool
)

func TestMain(m *testing.M) {
	var cfg testConfig
	err := config.LoadConfig("../..", &cfg)
	if err != nil {
		panic(err)
	}

	testPool, err = pgxpool.New(context.Background(), cfg.DBSource)
	if err != nil {
		panic(err)
	}
	defer testPool.Close()

	testStore = db.NewStore(testPool)
	os.Exit(m.Run())
}

func insertTestEvent(t *testing.T, eventType string, aggregateID uuid.UUID, payload []byte) db.OutboxEvent {
	t.Helper()
	event, err := testStore.InsertOutboxEvent(context.Background(), db.InsertOutboxEventParams{
		AggregateType: "user",
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
	})
	require.NoError(t, err)
	return event
}

func cleanupOutboxEvents(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `DELETE FROM outbox_events`)
	require.NoError(t, err)
}

func TestDispatchPublishesAndMarksPublished(t *testing.T) {
	cleanupOutboxEvents(t)
	defer cleanupOutboxEvents(t)

	userID := uuid.New()
	payload, err := json.Marshal(rabbitmq.UserCreated{ID: userID, Username: "alice"})
	require.NoError(t, err)
	event := insertTestEvent(t, "userCreated", userID, payload)

	sender := &recordingSender{}
	d := New(testStore, sender, Config{PollInterval: time.Hour})

	d.dispatchOnce(context.Background())

	require.Len(t, sender.queues, 1)
	require.Equal(t, rabbitmq.UserCreatedQueue, sender.queues[0])
	require.JSONEq(t, string(payload), string(sender.bodies[0]))

	events, err := testStore.ListUnpublishedOutboxEvents(context.Background(), 10)
	require.NoError(t, err)
	for _, e := range events {
		require.NotEqual(t, event.ID, e.ID, "published event should be marked")
	}
}

func TestDispatchKeepsEventOnPublishFailure(t *testing.T) {
	cleanupOutboxEvents(t)
	defer cleanupOutboxEvents(t)

	userID := uuid.New()
	payload, err := json.Marshal(rabbitmq.UserUpdated{ID: userID, NewUsername: "bob"})
	require.NoError(t, err)
	event := insertTestEvent(t, "userUpdated", userID, payload)

	sender := &recordingSender{err: errors.New("rabbitmq down")}
	d := New(testStore, sender, Config{PollInterval: time.Hour})

	d.dispatchOnce(context.Background())

	events, err := testStore.ListUnpublishedOutboxEvents(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, event.ID, events[0].ID, "failed event should be retried")
}

func TestQueueForEvent(t *testing.T) {
	require.Equal(t, rabbitmq.UserCreatedQueue, queueForEvent("userCreated"))
	require.Equal(t, rabbitmq.UserUpdatedQueue, queueForEvent("userUpdated"))
	require.Empty(t, queueForEvent("unknownEvent"))
}
