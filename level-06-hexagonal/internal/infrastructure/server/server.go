// Package server runs the HTTP server and shuts it down gracefully.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const shutdownTimeout = 10 * time.Second

// Run serves handler on addr until ctx is cancelled, then lets in-flight requests finish.
func Run(ctx context.Context, addr string, handler http.Handler, log zerolog.Logger) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Msg("listening")
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
