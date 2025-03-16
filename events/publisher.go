package events

import (
	"errors"
)

// ErrStreamClosed indicates that the stream has been closed.
var ErrStreamClosed = errors.New("stream closed")

// // ChannelPublisher is a helper for sending events to a Stream.
// type ChannelPublisher struct {
// 	stream    *ChannelStream
// 	closeOnce sync.Once
// 	closed    bool
// 	mutex     sync.Mutex
// }

// // newChannelPublisher returns a new ChannelPublisher for the given stream.
// func newChannelPublisher(stream *ChannelStream) *ChannelPublisher {
// 	return &ChannelPublisher{
// 		stream:    stream,
// 		closeOnce: sync.Once{},
// 	}
// }

// // Send sends an event to the stream's events channel
// func (p *ChannelPublisher) Send(ctx context.Context, event *Event) error {
// 	p.mutex.Lock()
// 	defer p.mutex.Unlock()

// 	if p.closed {
// 		return ErrStreamClosed
// 	}

// 	// Send the event, as long as the stream is open and the context
// 	// hasn't been canceled.
// 	select {
// 	case <-p.stream.done:
// 		p.close()
// 		return ErrStreamClosed

// 	case <-ctx.Done():
// 		p.close()
// 		return ctx.Err()

// 	case p.stream.events <- event:
// 		return nil
// 	}
// }

// // Close the publisher and close the corresponding Stream. No more calls to Send
// // should be made, however doing so will not cause a panic.
// func (p *ChannelPublisher) Close() {
// 	p.mutex.Lock()
// 	defer p.mutex.Unlock()

// 	p.close()
// }

// func (p *ChannelPublisher) close() {
// 	p.closeOnce.Do(func() {
// 		p.closed = true
// 		close(p.stream.events)
// 	})
// }
