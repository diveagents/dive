package dive

import (
	"context"
	"fmt"
	"time"

	"github.com/diveagents/dive/llm"
)

var ErrThreadNotFound = fmt.Errorf("thread not found")

// Thread represents a conversation thread
type Thread struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Messages  []*llm.Message `json:"messages"`
}

// ThreadRepository is an interface for storing and retrieving conversation threads
type ThreadRepository interface {

	// PutThread creates or updates a thread
	PutThread(ctx context.Context, thread *Thread) error

	// GetThread retrieves a thread by ID
	GetThread(ctx context.Context, id string) (*Thread, error)

	// DeleteThread deletes a thread by ID
	DeleteThread(ctx context.Context, id string) error
}
