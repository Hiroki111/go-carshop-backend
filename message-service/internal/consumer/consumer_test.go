package consumer_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/processor"
	"github.com/stretchr/testify/require"
)

const shortDelay = 5 * time.Millisecond

func TestRun_CommitsARecordAfterProcessingIt(t *testing.T) {
	f := setup(t, shortDelay)
	f.source.Enqueue(record("event-1"))
	stop := runInBackground(t, f.consumer)

	f.waitFor(t, "commit event-1", 1)
	stop()

	require.Equal(t, []string{"process event-1", "commit event-1"}, f.journal.Entries())
	require.Equal(t, []string{"event-1"}, valuesOfRecords(f.source.Committed()))
	require.Empty(t, f.dlq.Published())
}

func TestRun_HandlesRecordsOneAtATimeInTheOrderTheyArrive(t *testing.T) {
	f := setup(t, shortDelay)
	f.source.Enqueue(record("event-1"), record("event-2"), record("event-3"))
	stop := runInBackground(t, f.consumer)

	f.waitFor(t, "commit event-3", 1)
	stop()

	require.Equal(t, []string{
		"process event-1", "commit event-1",
		"process event-2", "commit event-2",
		"process event-3", "commit event-3",
	}, f.journal.Entries())
}

// A record that fails for a reason that may pass holds up the records behind
// it: they are not processed until it is done, and it is not committed until
// then either.
func TestRun_RetriesATransientFailureBeforeMovingOn(t *testing.T) {
	const delay = 20 * time.Millisecond
	f := setup(t, delay)
	f.processor.SetHandler(results(errTransient, errTransient))
	f.source.Enqueue(record("event-1"), record("event-2"))

	start := time.Now()
	stop := runInBackground(t, f.consumer)
	f.waitFor(t, "commit event-1", 1)
	elapsed := time.Since(start)
	f.waitFor(t, "commit event-2", 1)
	stop()

	require.Equal(t, []string{
		"process event-1", // fails
		"process event-1", // fails
		"process event-1", // succeeds
		"commit event-1",
		"process event-2",
		"commit event-2",
	}, f.journal.Entries())
	require.Empty(t, f.dlq.Published(), "a transient failure is not dead-lettered")
	require.GreaterOrEqual(t, elapsed, 2*delay, "the consumer waits before each retry")
}

func TestRun_DeadLettersARecordThatCanNeverSucceed(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "the error itself", err: processor.ErrPermanent},
		{name: "wrapped once", err: fmt.Errorf("%w: unknown event type", processor.ErrPermanent)},
		{name: "wrapped again", err: fmt.Errorf("handling record: %w", fmt.Errorf("%w: no such user", processor.ErrPermanent))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := setup(t, shortDelay)
			f.processor.SetHandler(results(test.err))
			f.source.Enqueue(record("event-1"), record("event-2"))
			stop := runInBackground(t, f.consumer)

			f.waitFor(t, "commit event-2", 1)
			stop()

			require.Equal(t, []string{
				"process event-1", // permanent failure
				"dead-letter event-1",
				"commit event-1",
				"process event-2",
				"commit event-2",
			}, f.journal.Entries(), "no retry, and the next record proceeds")

			published := f.dlq.Published()
			require.Len(t, published, 1)
			require.Equal(t, record("event-1").Key, published[0].key, "the dead letter keeps the original key")
			require.Equal(t, record("event-1").Value, published[0].value, "the dead letter keeps the original value")
			require.NotEmpty(t, published[0].reason, "whoever looks at the dead letter needs to know why")
		})
	}
}

// If the dead-letter topic can't take the record, the record must not be
// committed: committing it would lose the event.
func TestRun_DoesNotDropARecordWhenTheDeadLetterTopicIsDown(t *testing.T) {
	f := setup(t, shortDelay)
	f.processor.SetHandler(failWith(processor.ErrPermanent))
	f.dlq.FailPublishes(errBroker, errBroker)
	f.source.Enqueue(record("event-1"))

	start := time.Now()
	stop := runInBackground(t, f.consumer)
	f.waitFor(t, "commit event-1", 1)
	elapsed := time.Since(start)
	stop()

	entries := f.journal.Entries()
	require.Equal(t, 2, f.journal.Count("dead-letter event-1 (failed)"))
	require.Less(t, indexOf(t, entries, "dead-letter event-1"), indexOf(t, entries, "commit event-1"),
		"the record is committed only after it was dead-lettered")
	require.Equal(t, []string{"event-1"}, f.dlq.PublishedRecordValues())
	require.Equal(t, []string{"event-1"}, valuesOfRecords(f.source.Committed()))
	require.GreaterOrEqual(t, elapsed, 2*shortDelay, "the consumer waits before each retry")
}

// A failed commit is not a reason to stop or to do the work again. A later
// commit covers the earlier record; in the worst case the record is delivered
// once more after a restart, and the processor ignores duplicates.
func TestRun_KeepsGoingWhenACommitFails(t *testing.T) {
	f := setup(t, shortDelay)
	f.source.FailCommits(errBroker)
	f.source.Enqueue(record("event-1"), record("event-2"))
	stop := runInBackground(t, f.consumer)

	f.waitFor(t, "commit event-2", 1)
	stop()

	require.Equal(t, []string{"process event-1", "process event-2"}, withPrefix(f.journal.Entries(), "process "),
		"each record is processed once")
	require.Empty(t, f.dlq.Published())
	require.Equal(t, []string{"event-2"}, valuesOfRecords(f.source.Committed()))
}

// A broker outage must not kill the consumer: it tries again.
func TestRun_KeepsGoingWhenPollingFails(t *testing.T) {
	f := setup(t, shortDelay)
	f.source.EnqueueError(errBroker)
	f.source.EnqueueError(errBroker)
	f.source.Enqueue(record("event-1"))
	stop := runInBackground(t, f.consumer)

	f.waitFor(t, "commit event-1", 1)
	stop()

	require.Equal(t, []string{"process event-1", "commit event-1"}, f.journal.Entries())
}

func TestRun_StopsWhenCancelledWhileWaitingForARecord(t *testing.T) {
	f := setup(t, shortDelay)
	stop := runInBackground(t, f.consumer)
	require.Eventually(t, func() bool { return f.source.Polls() >= 1 }, 2*time.Second, 5*time.Millisecond)

	stop() // fails the test unless Run returns nil in good time

	require.Empty(t, f.journal.Entries())
}

// The retry delay is a long one here, so Run can only return in time if it
// stops waiting when it is cancelled.
func TestRun_StopsWhenCancelledWhileWaitingToRetry(t *testing.T) {
	f := setup(t, time.Hour)
	f.processor.SetHandler(failWith(errTransient))
	f.source.Enqueue(record("event-1"))
	stop := runInBackground(t, f.consumer)
	f.waitFor(t, "process event-1", 1)

	stop()

	require.Equal(t, []string{"process event-1"}, f.journal.Entries(), "the unfinished record is not committed")
}

// Shutting down interrupts the processor, which then fails because its context
// was cancelled. That says nothing about the record, so the record must not be
// committed, dead-lettered or tried again: the next run will receive it again.
func TestRun_LeavesARecordAloneWhenCancelledWhileProcessingIt(t *testing.T) {
	f := setup(t, shortDelay)
	f.processor.SetHandler(func(ctx context.Context, value string) error {
		<-ctx.Done()
		return fmt.Errorf("sending the email: %w", ctx.Err())
	})
	f.source.Enqueue(record("event-1"))
	stop := runInBackground(t, f.consumer)
	f.waitFor(t, "process event-1", 1)

	stop()

	require.Equal(t, []string{"process event-1"}, f.journal.Entries())
	require.Empty(t, f.dlq.Published())
	require.Empty(t, f.source.Committed())
}
