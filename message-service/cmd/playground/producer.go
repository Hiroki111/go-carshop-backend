package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runProducer(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	brokers := fs.String("brokers", defaultBrokers, "comma-separated bootstrap servers")
	topic := fs.String("topic", defaultTopic, "topic to write to")
	count := fs.Int("n", 12, "number of messages to send")
	orders := fs.Int("orders", 4, "number of distinct order IDs; message i belongs to order (i % orders) + 1")
	noKey := fs.Bool("no-key", false, "send messages without a key")
	_ = fs.Parse(args)

	if *count < 1 || *orders < 1 {
		return errors.New("-n and -orders must be at least 1")
	}

	// franz-go makes the producer idempotent by default (acks=all plus
	// producer IDs and sequence numbers), so a retry after a network error
	// can neither duplicate nor reorder messages. Nothing to configure here.
	cl, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(*brokers, ",")...))
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer cl.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// The position of each event within its own order: 1, 2, 3, ...
	seqOfOrder := map[int]int{}
	partitionsOfOrder := map[int]map[int32]bool{}

	for i := 0; i < *count; i++ {
		orderID := i%*orders + 1
		seqOfOrder[orderID]++

		rec := &kgo.Record{
			Topic: *topic,
			Value: []byte(fmt.Sprintf(`{"order_id":%d,"seq":%d}`, orderID, seqOfOrder[orderID])),
		}
		if !*noKey {
			// The key is what picks the partition: Kafka hashes it, so the
			// same order ID always lands on the same partition.
			rec.Key = []byte(strconv.Itoa(orderID))
		}

		if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
			return fmt.Errorf("produce message %d: %w", i+1, err)
		}

		fmt.Printf("sent  order=%d seq=%d -> partition=%d offset=%d\n",
			orderID, seqOfOrder[orderID], rec.Partition, rec.Offset)

		if partitionsOfOrder[orderID] == nil {
			partitionsOfOrder[orderID] = map[int32]bool{}
		}
		partitionsOfOrder[orderID][rec.Partition] = true
	}

	fmt.Println("\npartitions used by each order:")
	for orderID := 1; orderID <= *orders; orderID++ {
		var partitions []int
		for p := range partitionsOfOrder[orderID] {
			partitions = append(partitions, int(p))
		}
		sort.Ints(partitions)
		fmt.Printf("  order %d: %v\n", orderID, partitions)
	}
	return nil
}
