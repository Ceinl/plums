package streambridge

import (
	"context"
	"testing"
	"time"

	"github.com/Ceinl/plums/capabilities"
)

func TestForwardTranslatesAndCloses(t *testing.T) {
	in := make(chan int, 3)
	in <- 1
	in <- 2
	in <- 3
	close(in)

	out := Forward(context.Background(), in, func(i int) capabilities.StreamEvent {
		return capabilities.StreamEvent{Text: string(rune('0' + i))}
	})

	var got []string
	for ev := range out {
		got = append(got, ev.Text)
	}
	if len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Fatalf("forwarded events = %v, want [1 2 3]", got)
	}
}

func TestMapConvertsAndNeverNil(t *testing.T) {
	got := Map([]int{1, 2, 3}, func(i int) string { return string(rune('0' + i)) })
	if len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Fatalf("Map = %v, want [1 2 3]", got)
	}
	// Empty and nil input must still yield a non-nil slice (the List* contract).
	if out := Map(nil, func(i int) int { return i }); out == nil {
		t.Fatal("Map(nil) returned a nil slice, want non-nil empty")
	}
}

// TestForwardExitsOnCancelWhenConsumerStalls guards the goroutine-leak
// regression: with an unbuffered hand-off and no ctx select, a forwarder parked
// on a send to a consumer that stopped reading (stream cancelled / backend
// switched) blocked forever. Forward must observe ctx cancellation and exit even
// while parked on a full output buffer with the source channel still producing.
func TestForwardExitsOnCancelWhenConsumerStalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// More events than the output buffer, and never closed, so the forwarder must
	// park on a send once the buffer fills.
	in := make(chan int, bufferSize+10)
	for i := 0; i < bufferSize+10; i++ {
		in <- i
	}

	out := Forward(ctx, in, func(int) capabilities.StreamEvent {
		return capabilities.StreamEvent{Text: "x"}
	})

	// Let the forwarder fill the output buffer and park on the next send. We never
	// read out before cancelling, mirroring a consumer that walked away.
	time.Sleep(50 * time.Millisecond)
	cancel()
	// While the buffer stays full and unread, the only ready select case after
	// cancel is ctx.Done(), so the forwarder returns deterministically. Give it a
	// beat to do so before we start draining (draining first could keep the send
	// case ready and mask the leak).
	time.Sleep(50 * time.Millisecond)

	closed := make(chan struct{})
	go func() {
		for range out { //nolint:revive // drain buffered events until close
		}
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Forward did not exit after cancel — goroutine leaked")
	}
}
