package consumer_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/message-service/internal/consumer"
	"github.com/stretchr/testify/require"
)

var (
	errTransient = errors.New("user-service is unavailable")
	errBroker    = errors.New("broker unavailable")
)

// journal records, in order, everything the consumer does to its
// collaborators. The tests assert on it, because the order of "process",
// "commit" and "dead-letter" is the whole point of the consumer.
//
// Entries look like "process event-1", "commit event-1" and
// "dead-letter event-1". An attempt that failed gets " (failed)" appended.
type journal struct {
	mu      sync.Mutex
	entries []string
}

func (j *journal) add(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = append(j.entries, fmt.Sprintf(format, args...))
}

func (j *journal) Entries() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.entries)
}

func (j *journal) Count(entry string) int {
	return strings.Count("\n"+strings.Join(j.Entries(), "\n")+"\n", "\n"+entry+"\n")
}

// withPrefix returns the entries that start with prefix, in order.
func withPrefix(entries []string, prefix string) []string {
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			found = append(found, entry)
		}
	}
	return found
}

// indexOf returns the position of the first entry equal to want, and fails the
// test if there is none.
func indexOf(t *testing.T, entries []string, want string) int {
	t.Helper()

	i := slices.Index(entries, want)
	require.NotEqual(t, -1, i, "%q is missing from %v", want, entries)
	return i
}

// record builds a Kafka record whose value is its ID, which is also how the
// journal names it.
func record(id string) consumer.Record {
	return consumer.Record{Key: []byte("key-" + id), Value: []byte(id)}
}

func valuesOfRecords(records []consumer.Record) []string {
	values := make([]string, len(records))
	for i, record := range records {
		values[i] = string(record.Value)
	}
	return values
}

// fakeSource stands in for Kafka. Poll hands out the queued results one by
// one; once the queue is empty it blocks until the context is cancelled, like
// a real consumer waiting for new records.
type fakeSource struct {
	journal *journal

	mu         sync.Mutex
	queue      []pollResult
	commitErrs []error
	polls      int
	committed  []consumer.Record
}

type pollResult struct {
	record consumer.Record
	err    error
}

func (s *fakeSource) Enqueue(records ...consumer.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range records {
		s.queue = append(s.queue, pollResult{record: r})
	}
}

func (s *fakeSource) EnqueueError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = append(s.queue, pollResult{err: err})
}

// FailCommits makes the next commits fail with the given errors, one error per
// commit; a nil entry lets that commit succeed. Later commits succeed.
func (s *fakeSource) FailCommits(errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitErrs = errs
}

func (s *fakeSource) Polls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.polls
}

func (s *fakeSource) Committed() []consumer.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.committed)
}

func (s *fakeSource) Poll(ctx context.Context) (consumer.Record, error) {
	s.mu.Lock()
	s.polls++
	if len(s.queue) > 0 {
		next := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()
		return next.record, next.err
	}
	s.mu.Unlock()

	<-ctx.Done()
	return consumer.Record{}, ctx.Err()
}

func (s *fakeSource) Commit(ctx context.Context, r consumer.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.commitErrs) > 0 {
		err := s.commitErrs[0]
		s.commitErrs = s.commitErrs[1:]
		if err != nil {
			s.journal.add("commit %s (failed)", r.Value)
			return err
		}
	}
	s.committed = append(s.committed, r)
	s.journal.add("commit %s", r.Value)
	return nil
}

// fakeProcessor stands in for the message processor. By default every record
// succeeds; handle overrides that.
type fakeProcessor struct {
	journal *journal

	mu     sync.Mutex
	handle func(ctx context.Context, value string) error
}

func (p *fakeProcessor) SetHandler(handle func(ctx context.Context, value string) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handle = handle
}

func (p *fakeProcessor) Process(ctx context.Context, value []byte) error {
	p.journal.add("process %s", value)

	p.mu.Lock()
	handle := p.handle
	p.mu.Unlock()
	if handle == nil {
		return nil
	}
	return handle(ctx, string(value))
}

// results makes the nth call to Process (counting every record) return
// errs[n]; calls past the end succeed.
func results(errs ...error) func(context.Context, string) error {
	var mu sync.Mutex
	call := 0
	return func(ctx context.Context, value string) error {
		mu.Lock()
		defer mu.Unlock()
		defer func() { call++ }()
		if call < len(errs) {
			return errs[call]
		}
		return nil
	}
}

// failWith makes every call to Process fail with err.
func failWith(err error) func(context.Context, string) error {
	return func(context.Context, string) error { return err }
}

type deadLetter struct {
	key, value []byte
	reason     string
}

// fakeDeadLetterer stands in for the dead-letter topic. Only messages that were
// published successfully are kept in published.
type fakeDeadLetterer struct {
	journal *journal

	mu        sync.Mutex
	errs      []error
	published []deadLetter
}

// FailPublishes makes the next publishes fail with the given errors, one error
// per publish; a nil entry lets that publish succeed. Later publishes succeed.
func (d *fakeDeadLetterer) FailPublishes(errs ...error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.errs = errs
}

func (d *fakeDeadLetterer) Published() []deadLetter {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.published)
}

func (d *fakeDeadLetterer) Publish(ctx context.Context, key, value []byte, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.errs) > 0 {
		err := d.errs[0]
		d.errs = d.errs[1:]
		if err != nil {
			d.journal.add("dead-letter %s (failed)", value)
			return err
		}
	}
	d.published = append(d.published, deadLetter{key: key, value: value, reason: reason})
	d.journal.add("dead-letter %s", value)
	return nil
}

func (d *fakeDeadLetterer) PublishedRecordValues() []string {
	values := make([]string, len(d.published))
	for i, record := range d.published {
		values[i] = string(record.value)
	}
	return values
}

type fixture struct {
	consumer  *consumer.Consumer
	source    *fakeSource
	processor *fakeProcessor
	dlq       *fakeDeadLetterer
	journal   *journal
}

// setup returns a consumer over fakes. Nothing is queued yet and every record
// would succeed. retryDelay is how long the consumer waits before trying again.
func setup(t *testing.T, retryDelay time.Duration) *fixture {
	t.Helper()

	j := &journal{}
	source := &fakeSource{journal: j}
	proc := &fakeProcessor{journal: j}
	dlq := &fakeDeadLetterer{journal: j}

	return &fixture{
		consumer:  consumer.New(source, proc, dlq, consumer.Config{RetryDelay: retryDelay}),
		source:    source,
		processor: proc,
		dlq:       dlq,
		journal:   j,
	}
}

// waitFor blocks until the journal holds the entry at least n times.
func (f *fixture) waitFor(t *testing.T, entry string, n int) {
	t.Helper()

	require.Eventually(t, func() bool {
		return f.journal.Count(entry) >= n
	}, 2*time.Second, 5*time.Millisecond, "waiting for %q x%d; journal: %v", entry, n, f.journal.Entries())
}

// runInBackground starts the consumer. The returned stop function cancels it
// and checks that it returns cleanly, and quickly; it is also called when the
// test ends.
func runInBackground(t *testing.T, c *consumer.Consumer) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

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
