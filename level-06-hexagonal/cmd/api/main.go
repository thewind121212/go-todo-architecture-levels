// Command api serves the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level06/internal/config"
	"example.com/todo/level06/internal/infrastructure/logger"
)

const shutdownTimeout = 10 * time.Second

func main() {
	gin.SetMode(gin.ReleaseMode)
	log := logger.New(os.Stdout)
	if err := run(log); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}

func run(log zerolog.Logger) error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, cleanup, err := newServer(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer cleanup()

	serveErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Str("db_driver", cfg.DBDriver).Msg("listening")
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// newServer is where the runtime DB_DRIVER meets the compile-time Wire graphs:
// it picks the generated injector for the chosen storage.
func newServer(ctx context.Context, cfg config.Config, log zerolog.Logger) (*http.Server, func(), error) {
	switch cfg.DBDriver {
	case config.DriverPostgres:
		return initPostgresServer(ctx, cfg, log)
	case config.DriverMongo:
		return initMongoServer(ctx, cfg, log)
	default:
		return nil, nil, fmt.Errorf("unknown DB_DRIVER %q: use %q or %q", cfg.DBDriver, config.DriverPostgres, config.DriverMongo)
	}
}
