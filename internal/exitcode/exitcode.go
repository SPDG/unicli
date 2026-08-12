package exitcode

const (
	OK             = 0
	Usage          = 2
	Empty          = 3
	AuthRequired   = 4
	NotFound       = 5
	Permission     = 6
	RateLimited    = 7
	Retryable      = 8
	Config         = 10
	Unsupported    = 11
	MutationBlocked = 12
	InputRequired  = 13
	Cancelled      = 130
)
