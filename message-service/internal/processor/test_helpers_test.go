package processor_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/mailer"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/processor"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/userclient"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	eventConfirmed = "order.confirmed"
	eventCancelled = "order.cancelled"

	userID    = uint(7)
	userEmail = "alice@example.com"
	carName   = "Skoda Octavia"
)

type orderEvent struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	OrderID    uint      `json:"order_id"`
	UserID     uint      `json:"user_id"`
	CarID      uint      `json:"car_id"`
	CarName    string    `json:"car_name"`
	PriceCents uint      `json:"price_cents"`
}

func newEvent(eventID, eventType string) orderEvent {
	return orderEvent{
		EventID:    eventID,
		EventType:  eventType,
		Version:    1,
		OccurredAt: time.Now().UTC(),
		OrderID:    10,
		UserID:     userID,
		CarID:      5,
		CarName:    carName,
		PriceCents: 1234500,
	}
}

func encode(t *testing.T, event orderEvent) []byte {
	t.Helper()

	value, err := json.Marshal(event)
	require.NoError(t, err)
	return value
}

type fakeUserClient struct {
	emails map[uint]string
	err    error
	calls  []uint
}

func (f *fakeUserClient) GetUser(ctx context.Context, id uint) (userclient.User, error) {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return userclient.User{}, f.err
	}
	email, ok := f.emails[id]
	if !ok {
		return userclient.User{}, userclient.ErrUserNotFound
	}
	return userclient.User{ID: id, Email: email}, nil
}

type fakeMailer struct {
	err  error
	sent []mailer.Message
}

func (f *fakeMailer) Send(ctx context.Context, msg mailer.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}

type fixture struct {
	processor *processor.Processor
	users     *fakeUserClient
	mailer    *fakeMailer
	db        *gorm.DB
}

// setup returns a processor over an empty in-memory database, with user 7
// known to user-service.
func setup(t *testing.T) *fixture {
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

	userClient := &fakeUserClient{emails: map[uint]string{userID: userEmail}}
	mail := &fakeMailer{}

	return &fixture{
		processor: processor.New(repo, userClient, mail),
		users:     userClient,
		mailer:    mail,
		db:        db,
	}
}

func processedEventIDs(t *testing.T, db *gorm.DB) []string {
	t.Helper()

	var ids []string
	require.NoError(t, db.Model(&domain.ProcessedEvent{}).Order("event_id").Pluck("event_id", &ids).Error)
	return ids
}
