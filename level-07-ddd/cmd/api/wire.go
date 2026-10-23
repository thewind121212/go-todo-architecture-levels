//go:build wireinject

package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"example.com/todo/level07/internal/config"
	providers "example.com/todo/level07/internal/wire"
)

// Injectors: Wire reads these signatures and writes their bodies into
// wire_gen.go. A Wire graph is fixed at compile time, but DB_DRIVER is only
// known at runtime, so there is one injector per storage choice and main
// picks one.

func initPostgresApp(ctx context.Context, cfg config.Config, log zerolog.Logger) (*gin.Engine, func(), error) {
	wire.Build(providers.PostgresSet)
	return nil, nil, nil
}

func initMongoApp(ctx context.Context, cfg config.Config, log zerolog.Logger) (*gin.Engine, func(), error) {
	wire.Build(providers.MongoSet)
	return nil, nil, nil
}
