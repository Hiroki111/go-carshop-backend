package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/mailer"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/message-service/internal/userclient"
)

type Processor struct {
	repo       *repository.Repository
	userClient userclient.UserClient
	mailer     mailer.Mailer
}

func New(repo *repository.Repository, userClient userclient.UserClient, mailer mailer.Mailer) *Processor {
	return &Processor{
		repo:       repo,
		userClient: userClient,
		mailer:     mailer,
	}
}

// Process handles one message from the orders topic. It returns nil when the
// customer has been emailed, and also when the event was handled before. An
// error wrapping ErrPermanent means the event can never succeed, so it should
// be dead-lettered; any other error may go away by itself, so the event should
// be tried again later.
func (p *Processor) Process(ctx context.Context, value []byte) error {
	var orderEvent OrderEvent
	if err := json.Unmarshal(value, &orderEvent); err != nil {
		return fmt.Errorf("%w: invalid JSON: %v", ErrPermanent, err)
	}

	if !validEventTypes[orderEvent.EventType] {
		return fmt.Errorf("%w: unknown event type %q", ErrPermanent, orderEvent.EventType)
	}

	if orderEvent.EventID == "" {
		return fmt.Errorf("%w: missing event ID", ErrPermanent)
	}

	if orderEvent.UserID == 0 {
		return fmt.Errorf("%w: event %s has no user ID", ErrPermanent, orderEvent.EventID)
	}

	processed, err := p.repo.IsEventProcessed(orderEvent.EventID)
	if err != nil {
		return fmt.Errorf("check whether event %s was processed: %w", orderEvent.EventID, err)
	}
	if processed {
		log.Printf("event %s was already processed, skipping it", orderEvent.EventID)
		return nil
	}

	user, err := p.userClient.GetUser(ctx, orderEvent.UserID)
	if errors.Is(err, userclient.ErrUserNotFound) {
		return fmt.Errorf("%w: event %s: %w", ErrPermanent, orderEvent.EventID, err)
	}
	if err != nil {
		return fmt.Errorf("event %s: get user %d: %w", orderEvent.EventID, orderEvent.UserID, err)
	}

	// NOTE: a crash between sending the email and recording the event causes a
	// duplicate email on redelivery. This can't be closed, because SMTP and the
	// database share no transaction.
	if err := p.mailer.Send(ctx, buildEmail(user, orderEvent)); err != nil {
		return fmt.Errorf("event %s: send email: %w", orderEvent.EventID, err)
	}

	if err := p.repo.CreateProcessedEvent(orderEvent.EventID); err != nil {
		return fmt.Errorf("event %s: record it as processed: %w", orderEvent.EventID, err)
	}

	return nil
}

func buildEmail(user userclient.User, orderEvent OrderEvent) mailer.Message {
	email := mailer.Message{To: user.Email}

	switch orderEvent.EventType {
	case EventConfirmed:
		email.Subject = "Thank you for your order!"
		email.Body = fmt.Sprintf("Hi %v,\n\n"+
			"We received your order for %v. We'll get back to you as soon as the dealer acknowledges the order. "+
			"If you don't hear from us within 3 working days, contact us at +012 345 6789.", user.UserName, orderEvent.CarName)
	case EventCancelled:
		email.Subject = "Order cancellation"
		email.Body = fmt.Sprintf("Hi %v,\n\n"+
			"We received your order cancellation request for %v.", user.UserName, orderEvent.CarName)
	}
	return email
}
