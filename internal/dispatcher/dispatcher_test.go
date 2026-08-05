package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
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
	mu                sync.Mutex
	queues            []string
	bodies            [][]byte
	err               error
	errForBody        map[string]error
	firstPublishDelay time.Duration
	sent              int
}

func (s *recordingSender) SendMessage(_ context.Context, queue string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}
	if err := s.errForBody[string(body)]; err != nil {
		return err
	}
	if s.firstPublishDelay > 0 && s.sent == 0 {
		time.Sleep(s.firstPublishDelay)
	}
	s.sent++
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

	require.True(t, d.dispatchOnce(context.Background()))

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

	require.False(t, d.dispatchOnce(context.Background()))

	events, err := testStore.ListUnpublishedOutboxEvents(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, event.ID, events[0].ID, "failed event should be retried")
}

func TestDispatchContinuesBatchAfterPoisonRow(t *testing.T) {
	cleanupOutboxEvents(t)
	defer cleanupOutboxEvents(t)

	okID := uuid.New()
	poisonID := uuid.New()
	okPayload, err := json.Marshal(rabbitmq.UserCreated{ID: okID, Username: "carol"})
	require.NoError(t, err)
	poisonPayload, err := json.Marshal(rabbitmq.UserCreated{ID: poisonID, Username: "poison"})
	require.NoError(t, err)
	okEvent := insertTestEvent(t, "userCreated", okID, okPayload)
	poisonEvent := insertTestEvent(t, "userCreated", poisonID, poisonPayload)

	poison := string(poisonEvent.Payload)
	sender := &recordingSender{errForBody: map[string]error{poison: errors.New("poison")}}
	d := New(testStore, sender, Config{PollInterval: time.Hour})

	require.False(t, d.dispatchOnce(context.Background()))

	events, err := testStore.ListUnpublishedOutboxEvents(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1, "poison row must stay unpublished")
	require.Equal(t, poisonEvent.ID, events[0].ID)

	remaining, err := testStore.GetOutboxEventById(context.Background(), okEvent.ID)
	require.NoError(t, err)
	require.True(t, remaining.PublishedAt.Valid, "healthy event must be marked published")
}

func TestConcurrentDispatchersPublishEachEventOnce(t *testing.T) {
	cleanupOutboxEvents(t)
	defer cleanupOutboxEvents(t)

	const n = 10
	ids := make(map[string]uuid.UUID, n)
	for i := 0; i < n; i++ {
		id := uuid.New()
		ids[id.String()] = id
		payload, err := json.Marshal(rabbitmq.UserCreated{ID: id, Username: id.String()})
		require.NoError(t, err)
		insertTestEvent(t, "userCreated", id, payload)
	}

	sender := &recordingSender{firstPublishDelay: 200 * time.Millisecond}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		d := New(testStore, sender, Config{PollInterval: time.Hour})
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			d.dispatchOnce(context.Background())
		}()
	}
	close(start)
	wg.Wait()

	require.Len(t, sender.queues, n, "each event must be published exactly once")
	for _, body := range sender.bodies {
		var created rabbitmq.UserCreated
		require.NoError(t, json.Unmarshal(body, &created))
		_, ok := ids[created.ID.String()]
		require.True(t, ok, "unexpected event published")
	}
	require.Len(t, sender.queues, len(ids), "no event may be published twice")
}

func TestQueueForEvent(t *testing.T) {
	require.Equal(t, rabbitmq.UserCreatedQueue, queueForEvent("userCreated"))
	require.Equal(t, rabbitmq.UserUpdatedQueue, queueForEvent("userUpdated"))
	require.Empty(t, queueForEvent("unknownEvent"))
}
