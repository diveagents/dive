package stream

// Stream is an implementation of the Stream interface. It is used to stream
// task events from a team or agent to a client.
type Stream struct {
	events chan *Event
	done   chan struct{} // Signal channel for shutdown
}

// New returns a new Stream. Typically, a stream is created for each
// task or group of tasks that is to be executed.
func New() *Stream {
	return &Stream{
		events: make(chan *Event, 16),
		done:   make(chan struct{}),
	}
}

// Channel returns a channel that can be used by the client to receive events.
func (s *Stream) Channel() <-chan *Event {
	return s.events
}

// Close is used by the client to indicate that it no longer wishes to receive
// events, even if the task is not yet done. Any publisher should monitor the
// done channel and stop sending events when it is closed.
func (s *Stream) Close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
}
