// Command playground is a throwaway program for trying Kafka by hand. It is
// not part of message-service itself and will be deleted once the real
// consumer exists.
//
// It has two sub-commands, run from the message-service directory:
//
//	go run ./cmd/playground produce [flags]
//	go run ./cmd/playground consume [flags]
//
// Both talk to the broker published by docker-compose on localhost:29092 and
// use the topic "orders" (3 partitions). See each command's -h for its flags.
//
// Experiments (run each consumer in its own terminal):
//
//  1. Keys and partitions. Run `produce`, and watch which partition each order
//     lands on. All messages of one order go to the same partition, and their
//     offsets increase. Run it again with -no-key: the messages are spread
//     around and there is no ordering between them.
//
//  2. Consuming in a group. Run `consume -group g1`, then `produce`. The
//     consumer prints each message and commits its offset only after
//     handling it. Stop it (Ctrl-C) and start it again with the same group:
//     it prints nothing, because everything is committed. Run `produce`
//     again and only the new messages arrive. Start it with a different
//     group name (-group g2) and it reads everything from the beginning:
//     each group has its own offsets, and consuming deletes nothing.
//
//  3. Redelivery. Run `consume -group g3 -no-commit`, then `produce`, then
//     Ctrl-C. Start it again with the same flags: the same messages are
//     delivered again, because nothing was committed. This is "at least once".
//
//  4. Two consumers, one group. Run `consume -group g4 -delay 1s` in two
//     terminals, then `produce -n 30`. Each consumer gets some partitions.
//     Watch the "assigned" and "revoked" lines. Ctrl-C one of them and the
//     other picks up its partitions. Start a third and one partition
//     moves to it. Start a fourth and it sits idle: 3 partitions means at
//     most 3 busy consumers in a group.
//
//  5. A failing handler. Run `consume -group g5 -fail-key 2`, then `produce`.
//     Order 2's first message keeps failing and is retried forever, and
//     every message behind it (in the same fetch, including those from other
//     partitions) waits. Ctrl-C and restart without -fail-key: the stuck
//     message is delivered again and the backlog drains. In the real service
//     this is why a message that can never succeed goes to a dead-letter
//     topic instead of being retried for ever.
package main

import (
	"fmt"
	"os"
)

const (
	defaultBrokers = "localhost:29092"
	defaultTopic   = "orders"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "produce":
		err = runProducer(os.Args[2:])
	case "consume":
		err = runConsumer(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: playground produce|consume [flags]   (add -h after the command for its flags)")
}
