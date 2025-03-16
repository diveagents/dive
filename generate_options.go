package dive

type GenerateOptions struct {
	ThreadID string
	UserID   string
}

type GenerateOption func(*GenerateOptions)

func WithThreadID(threadID string) GenerateOption {
	return func(opts *GenerateOptions) {
		opts.ThreadID = threadID
	}
}

func WithUserID(userID string) GenerateOption {
	return func(opts *GenerateOptions) {
		opts.UserID = userID
	}
}

func (o *GenerateOptions) Apply(opts []GenerateOption) {
	for _, opt := range opts {
		opt(o)
	}
}
