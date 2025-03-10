package agent

type generateOptions struct {
	ThreadID string
	UserID   string
}

type GenerateOption func(*generateOptions)

func WithThreadID(threadID string) GenerateOption {
	return func(opts *generateOptions) {
		opts.ThreadID = threadID
	}
}

func WithUserID(userID string) GenerateOption {
	return func(opts *generateOptions) {
		opts.UserID = userID
	}
}

func (o *generateOptions) Apply(opts []GenerateOption) {
	for _, opt := range opts {
		opt(o)
	}
}
