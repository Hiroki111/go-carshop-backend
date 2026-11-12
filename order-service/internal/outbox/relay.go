package outbox

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
)

type Publisher interface {
	Publish(ctx context.Context, key string, value []byte) error
}

type Config struct {
	BatchSize    int
	PollInterval time.Duration
}

type Relay struct {
	repo      *repository.Repository
	publisher Publisher
	cfg       Config
}

const (
	defaultBatchSize    = 100
	defaultPollInterval = time.Second
)

func NewRelay(repo *repository.Repository, publisher Publisher, cfg Config) *Relay {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &Relay{
		repo:      repo,
		publisher: publisher,
		cfg:       Config{BatchSize: batchSize, PollInterval: pollInterval},
	}
}

func (r *Relay) RunOnce(ctx context.Context) error {
	events, err := r.repo.GetUnpublishedOutboxEvents(r.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get outbox events: %w", err)
	}
	for _, event := range events {
		if err := r.publisher.Publish(ctx, strconv.FormatUint(uint64(event.OrderID), 10), []byte(event.Payload)); err != nil {
			return fmt.Errorf("failed to publish event %d: %w", event.ID, err)
		}
		if err := r.repo.MarkOutboxEventPublished(event.ID); err != nil {
			return fmt.Errorf("failed to mark event published %d: %w", event.ID, err)
		}
	}
	return nil
}

func (r *Relay) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(r.cfg.PollInterval):
		}
		if err := r.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("outbox relay: %v", err)
		}
	}
}
