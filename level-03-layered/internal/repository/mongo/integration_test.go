//go:build integration

package mongo_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"

	"example.com/todo/level03/internal/repository/mongo"
	"example.com/todo/level03/internal/repository/repotest"
)

func TestContract(t *testing.T) {
	// Transactions need a replica set, even a single-node one.
	ctr, err := mongodb.Run(t.Context(), "mongo:8", mongodb.WithReplicaSet("rs0"))
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start mongo: %v", err)
	}
	uri, err := ctr.ConnectionString(t.Context())
	if err != nil {
		t.Fatalf("uri: %v", err)
	}
	// The replica set advertises a container-internal host, so connect directly.
	client, err := mongo.Open(uri + "&directConnection=true")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Disconnect(context.Background()) }) // t.Context() is already canceled here
	db := client.Database("todo_test")
	if err := mongo.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}

	repotest.Run(t, func(t *testing.T) repotest.Storage {
		for _, name := range []string{"todos", "activities"} {
			if _, err := db.Collection(name).DeleteMany(t.Context(), bson.D{}); err != nil {
				t.Fatalf("clear %s: %v", name, err)
			}
		}
		return repotest.Storage{
			Tx:         mongo.NewTxManager(client),
			Todos:      mongo.NewTodoRepository(db),
			Activities: mongo.NewActivityRepository(db),
		}
	})
}
