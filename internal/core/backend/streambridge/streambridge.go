// Package streambridge holds the shared translation helpers every backend
// bridge uses to convert a provider's native types into the public capabilities
// types: Forward for the streaming event channel, Map for List* slice results.
package streambridge

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
)

// bufferSize decouples the forwarding goroutine from the UI consumer so a brief
// stall (a synchronous question reply, a post-stream model refresh) does not
// back-pressure the provider's reader and stall its tool calls.
const bufferSize = 256

// Forward translates every event from in through conv onto a buffered output
// channel, closing it when in is drained. It selects on ctx.Done() for each send
// so that when the consumer stops reading (stream cancelled, backend switched)
// the goroutine exits instead of blocking forever on the send — the provider
// shares ctx and closes in on the same cancellation.
func Forward[T any](ctx context.Context, in <-chan T, conv func(T) capabilities.StreamEvent) <-chan capabilities.StreamEvent {
	out := make(chan capabilities.StreamEvent, bufferSize)
	go func() {
		defer close(out)
		for event := range in {
			select {
			case <-ctx.Done():
				return
			case out <- conv(event):
			}
		}
	}()
	return out
}

// Ptr converts an optional value: nil in yields nil out, otherwise conv is
// applied and its result returned by address. It collapses the bridges' repeated
// nil-guarded pointer wrappers (a *provider.Session into a *capabilities.Session).
func Ptr[T, U any](v *T, conv func(T) U) *U {
	if v == nil {
		return nil
	}
	out := conv(*v)
	return &out
}

// Map converts each element of in through conv. It always returns a non-nil
// slice (empty for empty/nil input), matching the bridges' List* contract of
// never handing the runtime a nil slice on success.
func Map[T, U any](in []T, conv func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, conv(v))
	}
	return out
}
