//go:build integration

package activity

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"example.com/todo/level04/internal/platform/testdb"
)

func TestPostgres(t *testing.T) {
	db := testdb.Postgres(t)
	if err := MigratePostgres(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	runContract(t, NewPostgresRepository(db))
}

func TestMongo(t *testing.T) {
	db := testdb.Mongo(t).Database("todo")
	if err := MigrateMongo(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	runContract(t, NewMongoRepository(db))
}

// runContract is the behavior both repositories must share.
func runContract(t *testing.T, repo Repository) {
	ctx := context.Background()

	t.Run("lists one todo's activities oldest first", func(t *testing.T) {
		todoA, todoB := uuid.NewString(), uuid.NewString()
		var want []Activity
		for _, action := range []string{"created", "renamed", "completed"} {
			a, err := New(todoA, action)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Create(ctx, a); err != nil {
				t.Fatalf("Create: %v", err)
			}
			want = append(want, a)
		}
		other, _ := New(todoB, "created")
		if err := repo.Create(ctx, other); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.ListByTodo(ctx, todoA)
		if err != nil {
			t.Fatalf("ListByTodo: %v", err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d activities, want %d: %+v", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("activity %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("unknown todo has an empty list", func(t *testing.T) {
		got, err := repo.ListByTodo(ctx, uuid.NewString())
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("ListByTodo = %v, %v; want empty, non-nil", got, err)
		}
	})
}
