package ai

import (
	"context"
	"math/rand"
	"time"
)

func SudoText() string {
	r := rand.Intn(26)
	if r == 0 {
		time.Sleep(10 * time.Millisecond)
		return " "
	}
	return string(rune('a' + r - 1))
}

func RepeatFunc[T any](ctx context.Context, fn func() T) <-chan T {
	stream := make(chan T)
	go func() {
		defer close(stream)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			val := fn()

			select {
			case <-ctx.Done():
				return
			case stream <- val:
			}
		}
	}()
	return stream
}
