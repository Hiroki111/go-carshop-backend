package mailer

import "context"

type Message struct {
	To, Subject, Body string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
