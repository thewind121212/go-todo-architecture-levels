# Level 1: single file

## What this level teaches

The whole app (domain rules, two storage adapters, HTTP and wiring) lives in one `main.go` that reads top to bottom.
It shows the smallest useful abstraction: a `store` interface with one Postgres and one Mongo implementation, picked at startup by `DB_DRIVER`.
It also shows the core lesson of the series in its simplest form: every write saves the todo and its activity rows in **one transaction**, and that transaction is opened inside each store method.

## Folder tree

```
level-01-single-file/
  main.go              the entire app, in sections: domain, storage: postgres, storage: mongo, http, main
  main_test.go         unit tests: validation and applyPatch tables, handler tests with an in-memory fake store
  integration_test.go  one contract suite run against Postgres and Mongo in testcontainers (build tag `integration`)
  go.mod / go.sum      independent module example.com/todo/level01
  README.md            this file
```

## Request flow

`PATCH /todos/:id` with `{"title":"new","completed":true}`. Everything is in `main.go`:

1. **`newRouter`** (`// --- http ---`) routes the request to `handler.updateTodo`.
2. **`updateTodo`** calls `todoID` (a malformed UUID becomes `ErrNotFound`), binds the JSON into a `Patch`,
   then calls `validatePatch` (`// --- domain ---`), which trims the title and rejects an empty patch.
3. It calls `h.store.Update(ctx, id, patch)`. The handler only knows the `store` interface.
4. **`postgresStore.Update`** (or `mongoStore.Update`) opens a transaction, reads the current todo inside it,
   and calls **`applyPatch`**. This shared domain function changes the todo and returns one `Activity` per real change
   (`renamed`, then `completed`). The store saves the todo and inserts the activities, then commits.
5. Back in the handler, success writes `200` with the todo. Any error goes to **`writeError`**,
   the one place that maps `ErrNotFound` to 404, validation errors to 400, and everything else to a logged 500.

## Where the transaction lives

Inside each store method. Each store has a tiny `inTx` helper, and `applyPatch` holds the shared rule:

```go
func (s *postgresStore) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	var t Todo
	err := s.inTx(ctx, func(tx *gorm.DB) error { // BEGIN
		var err error
		t, err = findTodo(tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
		if err != nil {
			return err // ROLLBACK
		}
		activities := applyPatch(&t, p, nowUTC())
		if len(activities) == 0 {
			return nil
		}
		if err := tx.Save(&t).Error; err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if err := tx.Create(&activities).Error; err != nil {
			return fmt.Errorf("insert activities: %w", err)
		}
		return nil // COMMIT
	})
	...
}
```

Mongo does the same with a session. Every collection call must use the `ctx` that `WithTransaction` passes in,
because that ctx carries the session:

```go
func (s *mongoStore) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := s.client.StartSession()
	...
	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}
```

`SELECT ... FOR UPDATE` stops two concurrent PATCHes from both logging `completed`.
Mongo gets the same safety from write conflicts: `WithTransaction` reruns `fn` when another transaction got there first.

## Pros / cons / when to use this in real life

**Pros**
- Nothing to navigate: one file, one package, `go run .` and you are done.
- Every step of a request is visible in one scroll, which makes it easy to learn and debug.
- The `store` interface already makes the database swappable and the handlers unit-testable with a fake.

**Cons** (these are what levels 2 and 3 fix)
- One file mixes HTTP, business rules, and SQL/BSON. At 600+ lines it is already hard to find things,
  and every change touches the same file, so merge conflicts are guaranteed on a team.
- One `Todo` type carries `json`, `gorm` and `bson` tags at once. A change to the API shape or a storage
  schema leaks into every other layer.
- The store owns the business flow. "Read the todo, apply the patch, save it, log activities" is written
  twice (once per database), and each store decides when the transaction starts. Adding a rule such as
  "notify on complete" means editing both stores.
- Business logic is hard to test without a DB. `applyPatch` is pure and easy to test, but the flow around it
  (not found, no-op patch, which activities get saved) only runs inside the real stores. The fake store has to copy
  that flow, so the handler tests check the fake's version of the rules, not the real one.

**When to use it:** prototypes, spikes, tiny internal tools, a bug report reproduction, or a service you expect to
throw away. Split it up (level 2) as soon as it grows past a few hundred lines or a second person works on it.

## Run & test

From this directory, with `docker compose up -d` already running at the repo root:

```sh
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH

# run (http://localhost:8081)
DB_DRIVER=postgres go run .
DB_DRIVER=mongo go run .

# unit tests (no Docker)
go test ./...

# integration tests (start their own Postgres and Mongo containers)
go test -tags=integration ./...

# smoke test: start the app with either driver, then from the repo root
python3 scripts/smoke.py http://localhost:8081
```

Config (all optional): `DB_DRIVER` (`postgres` or `mongo`), `POSTGRES_DSN`, `MONGO_URI`, `MONGO_DB` (default `todo_l01`),
`HTTP_ADDR` (default `:8081`). Set `GIN_MODE=release` to silence Gin's debug lines.
