package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runConsumer(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	brokers := fs.String("brokers", defaultBrokers, "comma-separated bootstrap servers")
	topic := fs.String("topic", defaultTopic, "topic to read from")
	group := fs.String("group", "playground", "consumer group; each group has its own committed offsets")
	noCommit := fs.Bool("no-commit", false, "handle messages but never commit their offsets")
	failKey := fs.String("fail-key", "", "the handler fails for messages with this key (retried every second, forever)")
	delay := fs.Duration("delay", 0, "time the handler takes per message, e.g. 1s")
	_ = fs.Parse(args)

	if *group == "" {
		return errors.New("-group must not be empty")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(*brokers, ",")...),
		kgo.ConsumerGroup(*group),
		kgo.ConsumeTopics(*topic),

		// Only used when the group has no committed offset yet: start from
		// the oldest message instead of the newest.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),

		// By default the client commits offsets in the background, which
		// could mark a message as done before the handler has finished with
		// it. We commit by hand, after handling.
		kgo.DisableAutoCommit(),

		kgo.OnPartitionsAssigned(func(_ context.Context, _ *kgo.Client, assigned map[string][]int32) {
			fmt.Printf("-- assigned: %s\n", describePartitions(assigned))
		}),
		kgo.OnPartitionsRevoked(func(_ context.Context, _ *kgo.Client, revoked map[string][]int32) {
			fmt.Printf("-- revoked:  %s\n", describePartitions(revoked))
		}),
	)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	// Close leaves the group, which triggers a rebalance for the others right
	// away. Killing the process instead (kill -9) makes them wait for the
	// session timeout (45s by default) before they notice.
	defer cl.Close()

	fmt.Printf("consuming %q as group %q (Ctrl-C to stop)\n", *topic, *group)

	for {
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		for _, fe := range fetches.Errors() {
			fmt.Fprintf(os.Stderr, "fetch error (%s/%d): %v\n", fe.Topic, fe.Partition, fe.Err)
		}

		// Records are handled one at a time, in the order they arrived. For a
		// given partition that is the order they were written in; handling
		// one partition's records in parallel goroutines would lose that.
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			if !handleUntilSuccess(ctx, rec, *failKey, *delay) {
				return nil // interrupted while handling
			}

			if *noCommit {
				fmt.Printf("   (not committing p=%d offset=%d)\n", rec.Partition, rec.Offset)
				continue
			}

			// Committing offset N says "everything up to N is done" for this
			// partition. A fresh context is used so that Ctrl-C right after
			// handling doesn't lose this commit.
			commitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := cl.CommitRecords(commitCtx, rec)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "commit failed (p=%d offset=%d): %v\n", rec.Partition, rec.Offset, err)
			}
		}
	}
}

// handleUntilSuccess stands in for real work (sending an email, say). If the
// handler fails it is tried again, and nothing else is handled meanwhile,
// which is how a message that can never succeed blocks everything behind it.
// It returns false if ctx was cancelled before the handler succeeded.
func handleUntilSuccess(ctx context.Context, rec *kgo.Record, failKey string, delay time.Duration) bool {
	for attempt := 1; ; attempt++ {
		err := handle(ctx, rec, failKey, delay)
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}

		fmt.Printf("!! p=%d offset=%d key=%s failed (attempt %d): %v; retrying in 1s\n",
			rec.Partition, rec.Offset, rec.Key, attempt, err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
		}
	}
}

func handle(ctx context.Context, rec *kgo.Record, failKey string, delay time.Duration) error {
	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	if failKey != "" && string(rec.Key) == failKey {
		return errors.New("simulated failure")
	}

	fmt.Printf("ok p=%d offset=%d key=%s value=%s\n", rec.Partition, rec.Offset, rec.Key, rec.Value)
	return nil
}

// describePartitions turns {"orders": [2 0]} into "orders[0 2]".
func describePartitions(byTopic map[string][]int32) string {
	if len(byTopic) == 0 {
		return "nothing"
	}

	var parts []string
	for topic, partitions := range byTopic {
		sorted := append([]int32(nil), partitions...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		parts = append(parts, fmt.Sprintf("%s%v", topic, sorted))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
