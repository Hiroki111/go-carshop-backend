package processor_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/processor"
	"github.com/stretchr/testify/require"
)

func TestProcess_SendsOneEmailToTheCustomer(t *testing.T) {
	tests := []struct {
		testName  string
		eventType string
	}{
		{testName: "order confirmed", eventType: eventConfirmed},
		{testName: "order cancelled", eventType: eventCancelled},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			f := setup(t)

			err := f.processor.Process(context.Background(), encode(t, newEvent("event-1", test.eventType)))

			require.NoError(t, err)
			require.Equal(t, []uint{userID}, f.users.calls, "the customer is looked up by the event's user_id")
			require.Len(t, f.mailer.sent, 1)
			msg := f.mailer.sent[0]
			require.Equal(t, userEmail, msg.To)
			require.NotEmpty(t, msg.Subject)
			require.Contains(t, msg.Body, carName, "the email must say which car the order is about")
		})
	}
}

func TestProcess_RecordsTheEventAsProcessed(t *testing.T) {
	f := setup(t)

	require.NoError(t, f.processor.Process(context.Background(), encode(t, newEvent("event-1", eventConfirmed))))

	require.Equal(t, []string{"event-1"}, processedEventIDs(t, f.db))
}

func TestProcess_SameEventTwiceSendsOneEmail(t *testing.T) {
	f := setup(t)
	value := encode(t, newEvent("event-1", eventConfirmed))

	require.NoError(t, f.processor.Process(context.Background(), value))
	require.NoError(t, f.processor.Process(context.Background(), value))

	require.Len(t, f.mailer.sent, 1)
	require.Len(t, f.users.calls, 1)
	require.Equal(t, []string{"event-1"}, processedEventIDs(t, f.db))
}

func TestProcess_ConfirmedThenCancelledSendsBothEmails(t *testing.T) {
	f := setup(t)

	require.NoError(t, f.processor.Process(context.Background(), encode(t, newEvent("event-1", eventConfirmed))))
	require.NoError(t, f.processor.Process(context.Background(), encode(t, newEvent("event-2", eventCancelled))))

	require.Len(t, f.mailer.sent, 2)
	require.NotEqual(t, f.mailer.sent[0].Subject, f.mailer.sent[1].Subject, "the customer can tell the two emails apart")
	require.Equal(t, []string{"event-1", "event-2"}, processedEventIDs(t, f.db))
}

func TestProcess_IgnoresFieldsItDoesNotKnow(t *testing.T) {
	f := setup(t)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encode(t, newEvent("event-1", eventConfirmed)), &fields))
	fields["coupon_code"] = "SPRING"
	value, err := json.Marshal(fields)
	require.NoError(t, err)

	require.NoError(t, f.processor.Process(context.Background(), value))

	require.Len(t, f.mailer.sent, 1)
}

func TestProcess_TransientFailureCanBeRetried(t *testing.T) {
	tests := []struct {
		testName string
		fail     func(f *fixture, err error)
	}{
		{testName: "user-service is unavailable", fail: func(f *fixture, err error) { f.users.err = err }},
		{testName: "the mail server is unavailable", fail: func(f *fixture, err error) { f.mailer.err = err }},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			f := setup(t)
			value := encode(t, newEvent("event-1", eventConfirmed))
			test.fail(f, errors.New("connection refused"))

			err := f.processor.Process(context.Background(), value)

			require.Error(t, err)
			require.NotErrorIs(t, err, processor.ErrPermanent, "a retry may still succeed")
			require.Empty(t, f.mailer.sent)
			require.Empty(t, processedEventIDs(t, f.db), "an unfinished event must not be recorded as done")

			test.fail(f, nil)
			require.NoError(t, f.processor.Process(context.Background(), value))

			require.Len(t, f.mailer.sent, 1)
			require.Equal(t, []string{"event-1"}, processedEventIDs(t, f.db))
		})
	}
}

func TestProcess_PermanentFailure(t *testing.T) {
	withChange := func(change func(*orderEvent)) []byte {
		event := newEvent("event-1", eventConfirmed)
		change(&event)
		return encode(t, event)
	}

	tests := []struct {
		testName         string
		value            []byte
		expectUserLookup bool
	}{
		{testName: "the user doesn't exist", value: withChange(func(e *orderEvent) { e.UserID = 99 }), expectUserLookup: true},
		{testName: "not valid JSON", value: []byte(`{"event_id": `)},
		{testName: "not JSON at all", value: []byte(`hello`)},
		{testName: "unknown event type", value: withChange(func(e *orderEvent) { e.EventType = "order.shipped" })},
		{testName: "missing event type", value: withChange(func(e *orderEvent) { e.EventType = "" })},
		{testName: "missing event ID", value: withChange(func(e *orderEvent) { e.EventID = "" })},
		{testName: "missing user ID", value: withChange(func(e *orderEvent) { e.UserID = 0 })},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			f := setup(t)

			err := f.processor.Process(context.Background(), test.value)

			require.ErrorIs(t, err, processor.ErrPermanent)
			require.Empty(t, f.mailer.sent)
			require.Empty(t, processedEventIDs(t, f.db))
			if test.expectUserLookup {
				require.Len(t, f.users.calls, 1)
			} else {
				require.Empty(t, f.users.calls, "an invalid event must be rejected before user-service is asked")
			}
		})
	}
}
