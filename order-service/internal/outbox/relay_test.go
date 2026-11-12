package outbox_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/outbox"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var errBroker = errors.New("broker unavailable")

type message struct {
	key   string
	value string
}

type fakePublisher struct {
	mu sync.Mutex
	// failWhen is asked about every attempt; a non-nil error makes that
	// attempt fail.
	failWhen func(message) error
	// attempts holds every call to Publish, failed or not; delivered only the
	// ones that were acknowledged.
	attempts  []message
	delivered []message
}

func (p *fakePublisher) Publish(ctx context.Context, key string, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg := message{key: key, value: string(value)}
	p.attempts = append(p.attempts, msg)
	if p.failWhen != nil {
		if err := p.failWhen(msg); err != nil {
			return err
		}
	}
	p.delivered = append(p.delivered, msg)
	return nil
}

func (p *fakePublisher) SetFailWhen(failWhen func(message) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failWhen = failWhen
}

func (p *fakePublisher) Attempts() []message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.attempts)
}

func (p *fakePublisher) Delivered() []message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.delivered)
}

// setup returns a relay over an empty in-memory database and a fake publisher.
func setup(t *testing.T, batchSize int) (*outbox.Relay, *fakePublisher, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := repository.NewRepository(db)
	require.NoError(t, repo.Migrate())

	publisher := &fakePublisher{}
	relay := outbox.NewRelay(repo, publisher, outbox.Config{
		BatchSize:    batchSize,
		PollInterval: 10 * time.Millisecond,
	})

	return relay, publisher, db
}

func newEvent(orderID uint, n int) domain.OutboxEvent {
	return domain.OutboxEvent{
		EventID:   fmt.Sprintf("event-%d", n),
		EventType: domain.EventTypeOrderConfirmed,
		OrderID:   orderID,
		Payload:   fmt.Sprintf(`{"order_id":%d,"n":%d}`, orderID, n),
	}
}

func seedEvents(t *testing.T, db *gorm.DB, events ...domain.OutboxEvent) []domain.OutboxEvent {
	t.Helper()

	for i := range events {
		require.NoError(t, db.Create(&events[i]).Error)
	}
	return events
}

func seedNewEvents(t *testing.T, db *gorm.DB, n int) []domain.OutboxEvent {
	t.Helper()

	events := make([]domain.OutboxEvent, n)
	for i := range events {
		events[i] = newEvent(uint(i%2)+1, i+1)
	}
	return seedEvents(t, db, events...)
}

func messageOf(event domain.OutboxEvent) message {
	return message{key: strconv.FormatUint(uint64(event.OrderID), 10), value: event.Payload}
}

func messagesOf(events []domain.OutboxEvent) []message {
	messages := make([]message, len(events))
	for i, event := range events {
		messages[i] = messageOf(event)
	}
	return messages
}

func idsOf(events []domain.OutboxEvent) []uint {
	ids := make([]uint, len(events))
	for i, event := range events {
		ids[i] = event.ID
	}
	return ids
}

func publishedIDs(t *testing.T, db *gorm.DB) []uint {
	t.Helper()

	var ids []uint
	require.NoError(t, db.Model(&domain.OutboxEvent{}).
		Where("published_at IS NOT NULL").Order("id").Pluck("id", &ids).Error)
	return ids
}

func TestRunOnce_PublishesEventsInTheOrderTheyWereRecorded(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	// Events of different orders are mixed on purpose: the relay publishes in
	// recording order, whatever the order ID is.
	events := seedEvents(
		t, db,
		newEvent(1, 1),
		newEvent(2, 2),
		newEvent(1, 3),
		newEvent(3, 4),
	)

	require.NoError(t, relay.RunOnce(context.Background()))

	require.Equal(t, messagesOf(events), publisher.Delivered())
	require.Equal(t, idsOf(events), publishedIDs(t, db))

	var stored []domain.OutboxEvent
	require.NoError(t, db.Order("id").Find(&stored).Error)
	for _, event := range stored {
		require.NotNil(t, event.PublishedAt)
		require.WithinDuration(t, time.Now(), *event.PublishedAt, time.Minute)
	}
}

// The ID is what defines the recording order: it is unique and increases with
// every insert, while timestamps can tie or be set by different clocks. Here
// the timestamps run backwards, so sorting by them would reverse the delivery.
func TestRunOnce_FollowsTheIDNotTheTimestamp(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	now := time.Now()
	first, second, third := newEvent(1, 1), newEvent(1, 2), newEvent(1, 3)
	first.CreatedAt = now
	second.CreatedAt = now.Add(-time.Hour)
	third.CreatedAt = now.Add(-2 * time.Hour)
	events := seedEvents(t, db, first, second, third)

	require.NoError(t, relay.RunOnce(context.Background()))

	require.Equal(t, messagesOf(events), publisher.Delivered())
}

func TestRunOnce_DoesNothingWhenNoEventIsWaiting(t *testing.T) {
	relay, publisher, _ := setup(t, 10)

	require.NoError(t, relay.RunOnce(context.Background()))

	require.Empty(t, publisher.Attempts())
}

func TestRunOnce_DoesNotSendPublishedEventsAgain(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	alreadyPublishedAt := time.Now().Add(-time.Hour)
	published := newEvent(1, 1)
	published.PublishedAt = &alreadyPublishedAt
	events := seedEvents(t, db, published, newEvent(2, 2), newEvent(1, 3))
	waiting := events[1:]

	require.NoError(t, relay.RunOnce(context.Background()))
	require.Equal(t, messagesOf(waiting), publisher.Delivered())

	// Running again finds nothing new.
	require.NoError(t, relay.RunOnce(context.Background()))
	require.Equal(t, messagesOf(waiting), publisher.Delivered())
}

// If an event can't be published, nothing recorded after it may be sent
// first: that would put the events of an order out of order in Kafka.
func TestRunOnce_StopsAtTheFirstFailure(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	events := seedNewEvents(t, db, 4)
	failing := messageOf(events[2])
	publisher.SetFailWhen(func(m message) error {
		if m == failing {
			return errBroker
		}
		return nil
	})

	require.ErrorIs(t, relay.RunOnce(context.Background()), errBroker)
	require.Equal(t, messagesOf(events[:3]), publisher.Attempts(), "the 4th event must not be tried")
	require.Equal(t, idsOf(events[:2]), publishedIDs(t, db), "events sent before the failure stay marked as published")
}

func TestRunOnce_ResumesAtTheEventThatFailed(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	events := seedNewEvents(t, db, 4)
	failing := messageOf(events[2])
	publisher.SetFailWhen(func(m message) error {
		if m == failing {
			return errBroker
		}
		return nil
	})
	require.ErrorIs(t, relay.RunOnce(context.Background()), errBroker)

	publisher.SetFailWhen(nil)
	require.NoError(t, relay.RunOnce(context.Background()))

	require.Equal(t, messagesOf(events), publisher.Delivered(), "every event is delivered once, in order")
	require.Equal(t, idsOf(events), publishedIDs(t, db))
}

func TestRunOnce_HandlesAtMostOneBatch(t *testing.T) {
	relay, publisher, db := setup(t, 2)
	events := seedNewEvents(t, db, 5)

	for _, expectedDelivered := range []int{2, 4, 5, 5} {
		require.NoError(t, relay.RunOnce(context.Background()))
		require.Equal(t, messagesOf(events[:expectedDelivered]), publisher.Delivered())
	}
}

// runInBackground starts relay.Run. The returned stop function cancels it and
// checks that it returns cleanly; it is also called when the test ends.
func runInBackground(t *testing.T, relay *outbox.Relay) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Run(ctx) }()

	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				require.NoError(t, err, "Run is expected to return nil when its context is cancelled")
			case <-time.After(2 * time.Second):
				t.Error("Run did not return after its context was cancelled")
			}
		})
	}
	t.Cleanup(stop)

	return stop
}

func TestRun_PublishesEventsRecordedAfterItStarted(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	stop := runInBackground(t, relay)

	events := seedNewEvents(t, db, 3)

	require.Eventually(t, func() bool {
		return len(publisher.Delivered()) == len(events)
	}, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, messagesOf(events), publisher.Delivered())

	stop()
}

func TestRun_KeepsGoingAfterAFailure(t *testing.T) {
	relay, publisher, db := setup(t, 10)
	events := seedNewEvents(t, db, 2)
	var failedOnce bool
	publisher.SetFailWhen(func(message) error {
		if !failedOnce {
			failedOnce = true
			return errBroker
		}
		return nil
	})
	stop := runInBackground(t, relay)

	require.Eventually(t, func() bool {
		return len(publisher.Delivered()) == len(events)
	}, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, messagesOf(events), publisher.Delivered())
	require.Len(t, publisher.Attempts(), 3, "one failed attempt, then each event once")

	stop()
}
