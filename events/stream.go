package events

// ChannelStream represents a stream of events.
type ChannelStream struct {
	events chan *Event
	done   chan struct{}
}

// NewStream returns a new ChannelStream.
func NewStream() *ChannelStream {
	return &ChannelStream{
		events: make(chan *Event),
		done:   make(chan struct{}),
	}
}

// Channel returns a channel that receives events from the stream.
func (s *ChannelStream) Channel() <-chan *Event {
	return s.events
}

// Close closes the stream.
func (s *ChannelStream) Close() {
	close(s.done)
}

// Publisher returns a publisher for the stream.
func (s *ChannelStream) Publisher() Publisher {
	return newChannelPublisher(s)
}
