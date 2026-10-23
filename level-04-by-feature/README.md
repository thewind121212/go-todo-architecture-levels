# Level 4: package by feature

## What this level teaches

The code from level 3 (handler, service, repository) is regrouped by **business feature** instead of by
technical layer: everything about todos lives in `internal/todo`, everything about the activity log in
`internal/activity`. The two features talk through a tiny interface that the *consumer* declares
(`todo.ActivityRecorder`), so `activity` never imports `todo` and `main` wires them together.
One transaction still spans both features: the todo service opens it, and both features' repositories
join it through the context.

## Folder tree

```
level-04-by-feature/
  cmd/api/main.go                 config from env, open DB, run migrations, wire features by hand, graceful shutdown
  internal/
    todo/                         the todo feature, all layers in one folder
      todo.go                     entity, title rules, Apply(patch) -> actions, sentinel errors, action names
      service.go                  use cases; declares the TxManager and ActivityRecorder interfaces it needs
      repository.go               Repository interface
      postgres.go                 GORM repository + MigratePostgres
      mongo.go                    Mongo repository + MigrateMongo
      handler.go                  Gin routes for /todos and writeError (the one error -> status mapping)
      fakes_test.go               hand-written fakes: repo, tx, activity recorder
      service_test.go             business rules against the fakes
      handler_test.go             HTTP tests with httptest
      integration_test.go         contract + cross-feature commit/rollback, Postgres and Mongo (tag: integration)
    activity/                     the activity feature; knows nothing about todo
      activity.go                 entity + constructor
      service.go                  Record (joins the caller's tx) and List
      repository.go               Repository interface
      postgres.go                 GORM repository + MigratePostgres (table + todo_id index)
      mongo.go                    Mongo repository + MigrateMongo (collection + todo_id index)
      handler.go                  GET /todos/:id/activities
      *_test.go                   unit, handler and integration tests, same pattern as todo
    platform/                     shared infrastructure only, no business code
      database/                   open Postgres / Mongo, EnsureCollection
      tx/                         PostgresManager, MongoManager, PostgresDB(ctx) helper
      httpx/                      InternalError: log the real error, answer {"error":"internal error"}
      testdb/                     testcontainers setup shared by both features' integration tests
  Taskfile.yml                    run:pg, run:mongo, test, test:integration, lint
```

## Request flow: `PATCH /todos/:id`

1. `cmd/api/main.go` registered `todo.Handler` on the Gin router.
2. `internal/todo/handler.go` `update` binds `{"title"?, "completed"?}` into a `todo.Patch`.
3. `internal/todo/service.go` `Update` rejects an empty patch, parses the id (malformed → `ErrNotFound`),
   then calls `tx.WithinTx`.
4. `internal/platform/tx/postgres.go` (or `mongo.go`) starts the transaction and puts it in `ctx`.
5. Inside the transaction the service calls `repo.Get` → `internal/todo/postgres.go`, which finds the
   transaction with `tx.PostgresDB(ctx, r.db)` (Mongo: the session already rides in `ctx`).
6. `internal/todo/todo.go` `Apply` validates the title and returns the actions, e.g. `[renamed completed]`.
7. `repo.Update` writes the todo; then, per action, `activities.Record(ctx, ...)`. That is
   `internal/activity/service.go` behind the `todo.ActivityRecorder` interface, writing through
   `internal/activity/postgres.go`, which joins **the same** transaction via the same ctx helper.
8. `fn` returns nil → commit. Any error → rollback, and `handler.go` `writeError` maps it to 400/404/500.

## Where the transaction lives

In the todo service, around writes that belong to two features:

```go
// internal/todo/service.go
err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
	t, err := s.repo.Get(ctx, id)
	...
	if err := s.repo.Update(ctx, t); err != nil {
		return err
	}
	for _, action := range actions {
		if err := s.activities.Record(ctx, t.ID, action); err != nil { // other feature, same tx
			return err
		}
	}
	return nil
})
```

The mechanism lives in `platform/tx`, so both features can use it without importing each other:

```go
// internal/platform/tx/postgres.go
func (m *PostgresManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(gtx *gorm.DB) error {
		return fn(context.WithValue(ctx, gormTxKey{}, gtx))
	})
}

// Every Postgres repository, in either feature, starts its queries here.
func PostgresDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if gtx, ok := ctx.Value(gormTxKey{}).(*gorm.DB); ok {
		return gtx
	}
	return db.WithContext(ctx)
}
```

For Mongo, `MongoManager.WithinTx` wraps `session.WithTransaction`; the ctx it hands to `fn` already
carries the session, so Mongo repositories need no helper.

## What changed from level 3

The same layers, sliced by feature instead of by layer:

```
level 3 (by layer)                         level 4 (by feature)
internal/                                  internal/
  model/todo.go, activity.go, errors.go      todo/      todo.go  service.go  repository.go
  service/todo_service.go                               postgres.go  mongo.go  handler.go
  repository/repository.go                   activity/  activity.go  service.go  repository.go
  repository/postgres/{db,todo_,activity_}               postgres.go  mongo.go  handler.go
  repository/mongo/{db,todo_,activity_}      platform/  database/  tx/  httpx/  testdb/
  handler/{router,todo_handler,dto,errors}
```

* **Cohesion.** A change to "todo" touches one folder. Deleting a feature means deleting a folder
  (plus its lines in `main.go`).
* **Activity is a real feature** with its own service and handler, instead of a repository the todo
  service writes to directly.
* **Consumer-side interface.** In level 3 the service used `repository.ActivityRepository` defined by the
  repository layer. Now `todo/service.go` declares only what it needs:
  `ActivityRecorder { Record(ctx, todoID, action string) error }`. `*activity.Service` satisfies it
  implicitly (Go interfaces need no `implements`), `main.go` passes one into the other, and tests use a
  small hand-written fake. The dependency arrow points one way only: `main → todo`, `main → activity`, never
  `activity → todo`.
* **Shared infra moves to `platform`.** The tx manager and its ctx helper used to live in each
  repository subpackage; now both features need them, so they live in `platform/tx`. There is no
  `shared`/`common`/`utils` package: `platform` holds infrastructure only.
* **Each feature migrates itself** (`todo.MigratePostgres`, `activity.MigrateMongo`, ...).
* The action names (`created`, `renamed`, ...) belong to `todo`, because they describe what happened
  to a todo; `activity` stores them as plain strings.

## Pros / cons / when to use this in real life

Pros
* Code you change together sits together; a feature folder is easy to read, review, own and delete.
* Package names become the domain vocabulary (`todo.Service`, `activity.Record`) instead of
  `service.TodoService`.
* Consumer-side interfaces keep features decoupled and make fakes trivial.

Cons
* Features still share one database and one transaction, so they are not independent deployables.
* Cross-feature calls need discipline: two features that both want to call each other create an import
  cycle, and you must invert one side with an interface.
* Inside a feature folder the layers are just files; nothing stops a handler from reaching into SQL.

When to use it: most small and medium Go services. It is the default layout recommended by many Go
teams once there is more than one business concept. Move to clean/hexagonal (level 5/6) when you need
the domain strictly isolated from frameworks, or to a modular monolith (level 9) when features need
their own data and transactions.

## Run & test

```sh
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH
docker compose up -d                 # once, at repo root

task run:pg                          # DB_DRIVER=postgres go run ./cmd/api   (:8084)
task run:mongo                       # DB_DRIVER=mongo    go run ./cmd/api   (:8084)

task test                            # go test ./...                      unit tests, no Docker
task test:integration                # go test -tags=integration ./...    testcontainers, both DBs + rollback
task lint                            # gofmt -l, go vet (both build tags)

# smoke test, from repo root, with the app running on either driver
python3 scripts/smoke.py http://localhost:8084
```

Config (env): `DB_DRIVER` (`postgres`|`mongo`), `POSTGRES_DSN`, `MONGO_URI`, `MONGO_DB` (`todo_l04`),
`HTTP_ADDR` (`:8084`).
