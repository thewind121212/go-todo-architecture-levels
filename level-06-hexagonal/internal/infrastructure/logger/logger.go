// Package logger builds the app's structured logger.
package logger

import (
	"io"

	"github.com/rs/zerolog"
)

// New returns a JSON logger with timestamps.
func New(w io.Writer) zerolog.Logger {
	return zerolog.New(w).With().Timestamp().Logger()
}
