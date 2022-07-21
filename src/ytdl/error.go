package ytdl

type Error struct {
	// `Input` is the input given to youtube-dl when the error occurred.
	// If `Input` is set to "", it is ignored.
	Input string
	// `Err` is the underlying error.
	Err error
}

// `input` may be set to "", in which case it is ignored when outputting the
// error message.
func newError(input string, err error) *Error {
	return &Error{
		Input: input,
		Err:   err,
	}
}

func (e *Error) Error() string {
	if e.Input == "" {
		return "ytdl: " + e.Err.Error()
	} else {
		return "ytdl['" + e.Input + "']: " + e.Err.Error()
	}
}
