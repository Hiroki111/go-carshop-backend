package consumer

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/processor"
)

type Consumer struct {
	src  Source
	proc Processor
	dlq  DeadLetterer
	cfg  Config
}

func New(src Source, proc Processor, dlq DeadLetterer, cfg Config) *Consumer {
	return &Consumer{
		src:  src,
		proc: proc,
		dlq:  dlq,
		cfg:  cfg,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		record, err := c.src.Poll(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("Polling a message failed. Retrying... Err: %v\n", err)
			}
			c.wait(ctx)
			continue
		}
		c.handle(ctx, record)
	}
	return nil
}

func (c *Consumer) handle(ctx context.Context, record Record) {
	for ctx.Err() == nil {
		processErr := c.proc.Process(ctx, record.Value)
		if processErr == nil {
			c.commitRecord(ctx, record)
			return
		}

		if !errors.Is(processErr, processor.ErrPermanent) {
			if ctx.Err() == nil {
				log.Printf("Processing a message (Key: %v) failed. Retrying... Err: %v\n", string(record.Key), processErr)
			}
			c.wait(ctx)
			continue
		}

		if ctx.Err() != nil {
			return
		}
		if dlqPublishErr := c.dlq.Publish(ctx, record.Key, record.Value, processErr.Error()); dlqPublishErr != nil {
			log.Printf("Publishing a dead-letter failed (Key:%v). Retrying... Err: %v\n", string(record.Key), dlqPublishErr)
			c.wait(ctx)
			continue
		}
		c.commitRecord(ctx, record)
		return
	}
}

func (c *Consumer) commitRecord(ctx context.Context, record Record) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := c.src.Commit(ctx, record); err != nil {
		log.Printf("Committing a record failed (Key:%v). Err: %v\n", string(record.Key), err)
	}
}

func (c *Consumer) wait(ctx context.Context) {
	select {
	case <-time.After(c.cfg.RetryDelay):
	case <-ctx.Done():
	}
}

type Record struct {
	Key, Value []byte
}

type Source interface {
	Poll(ctx context.Context) (Record, error)
	Commit(ctx context.Context, r Record) error
}

type DeadLetterer interface {
	Publish(ctx context.Context, key, value []byte, reason string) error
}

type Processor interface {
	Process(ctx context.Context, value []byte) error
}

type Config struct{ RetryDelay time.Duration }
