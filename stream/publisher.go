package stream

import (
	"context"
	"errors"
	"sync"
)

// ErrStreamClosed indicates that the stream has been closed.
var ErrStreamClosed = errors.New("stream closed")

// Publisher is a helper for sending events to a Stream.
type Publisher struct {
	stream    *Stream
	closeOnce sync.Once
	closed    bool
	mutex     sync.Mutex
}

// NewPublisher returns a new Publisher for the given stream.
func NewPublisher(stream *Stream) *Publisher {
	return &Publisher{
		stream:    stream,
		closeOnce: sync.Once{},
	}
}

// Send sends an event to the stream's events channel
func (p *Publisher) Send(ctx context.Context, event *Event) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return ErrStreamClosed
	}

	// Send the event, as long as the stream is open and the context
	// hasn't been canceled.
	select {
	case <-p.stream.done:
		p.close()
		return ErrStreamClosed

	case <-ctx.Done():
		p.close()
		return ctx.Err()

	case p.stream.events <- event:
		return nil
	}
}

// Close the publisher and close the corresponding Stream. No more calls to Send
// should be made, however doing so will not cause a panic.
func (p *Publisher) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.close()
}

func (p *Publisher) close() {
	p.closeOnce.Do(func() {
		p.closed = true
		close(p.stream.events)
	})
}
