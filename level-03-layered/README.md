# Level 3: classic layered (handler → service → repository)

## 1. What this level teaches

The code is split into three layers, and each one has a single job. The **handler** speaks HTTP, the **service** holds the business flow, and the **repositories** do plain single-table CRUD. Most importantly, the transaction moves out of storage and into the service: `TxManager.WithinTx` puts the transaction into the `context.Context`, and the repositories pick it up from there. The level also adds the first bits of production tooling: zerolog, a Taskfile and graceful shutdown.

## 2. Folder tree

```
level-03-layered/
  cmd/api/main.go                     config from env, open DB, wire layers by hand, http.Server + graceful shutdown
  internal/
    model/                            plain Todo/Activity types, domain errors, ValidatePatch, Todo.Apply (which activities a change produces)
    repository/
      repository.go                   the contracts: TodoRepository, ActivityRepository, TxManager
      postgres/                       GORM adapter: AutoMigrate, TxManager (tx in ctx), one repo per table
      mongo/                          driver v2 adapter: collections + index at startup, TxManager (session in ctx), one repo per collection
      repotest/                       contract suite both adapters run in their integration tests
      fake/                           hand-written in-memory repos + fake TxManager for unit tests
    service/todo_service.go           TodoService: business flows, opens the transactions
    handler/                          Gin routes, id/page parsing, DTOs, request logger, HTTP errors, writeError (the one error→status map)
  Taskfile.yml                        run:pg, run:mongo, test, test:integration, lint
```

## 3. Request flow: `PATCH /todos/:id`

1. `handler/router.go`: Gin matches the route; `requestLogger` will log method, path, status and latency when the request finishes.
2. `handler/todo_handler.go` `updateTodo`: `todoID` parses `:id` with `uuid.Parse` (a malformed id is `model.ErrNotFound`, so 404), then binds the JSON into `updateTodoRequest` (pointer fields, so a missing field is `nil`) and calls `TodoService.Update` with a `model.Patch`.
3. `service/todo_service.go` `Update`: first `model.ValidatePatch` (`model/todo.go`) rejects an empty patch and trims/validates the title, **before any database call**, so a bad body is 400 even for an unknown id. Then `tx.WithinTx` runs one transaction, and the callback uses the `ctx` it receives to:
   - lock and read the todo with `todos.GetForUpdate` (Postgres `SELECT … FOR UPDATE`),
   - call `todo.Apply(p, now)`, which only diffs: it returns `renamed` / `completed` / `reopened` for the fields that really changed,
   - save with `todos.Update` and insert one activity per action with `activities.Create`, all stamped with the same `now`.
4. `repository/postgres/*_repository.go` (or `repository/mongo/...`): each method runs a single statement. In Postgres `conn(ctx, db)` finds the GORM tx in `ctx`. In Mongo the `ctx` itself already carries the session.
5. Back in the service, `WithinTx` commits if the callback returned `nil` and rolls back otherwise.
6. `handler/todo_handler.go`: on success, maps `model.Todo` to `todoResponse` (`dto.go`) and returns 200. On error, `handler/errors.go` `writeError` maps it to 400/404/500 and logs any 500 with the real error.

## 4. Where the transaction lives

The **service** decides what belongs in one transaction (`internal/service/todo_service.go`):

```go
p, err := model.ValidatePatch(p) // bad input never opens a transaction
...
err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
    todo, err = s.todos.GetForUpdate(ctx, id)
    if err != nil {
        return err
    }
    now := nowUTC()
    actions := todo.Apply(p, now)
    if len(actions) == 0 {
        return nil // nothing changed, so nothing to write
    }
    if err := s.todos.Update(ctx, todo); err != nil {
        return err
    }
    return s.record(ctx, todo.ID, now, actions...)
})
```

The **adapters** only know how to start a transaction and pass it along. Postgres stores the `*gorm.DB` tx in `ctx`, and each repo picks it up (`internal/repository/postgres/db.go`):

```go
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
    return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return fn(context.WithValue(ctx, txKey{}, tx))
    })
}

func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
    if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
        return tx
    }
    return db.WithContext(ctx)
}
```

Mongo needs no `txKey` at all. Driver v2 already hands the callback a `ctx` that carries the session, and any collection call made with that `ctx` joins the transaction (`internal/repository/mongo/db.go`):

```go
_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
    return nil, fn(ctx)
})
```

That is why the Mongo repositories just use the `ctx` they get, while the Postgres ones call `conn(ctx, r.db)`. Mongo has no row lock either, so its `GetForUpdate` is a plain find. If a concurrent transaction writes the same todo, Mongo reports a write conflict and `WithTransaction` re-runs the whole callback. For that reason the callback must only touch the database.

## 5. What changed from level 2

Level 2 is level 1 split into files. There, `postgresStore` and `mongoStore` each implemented the whole flow behind one `store` interface: `Update(ctx, id, p)` opened its own `inTx`, read the row, called `applyPatch`, saved it and inserted the activities. So the business flow was written twice, once per database, and each copy was tangled with that database's transaction API. Now every layer has one job:

- **handler**: HTTP only. It parses the id and the page, binds JSON into its own DTOs, and has `writeError`. JSON tags no longer reach the model, and the one tagged `Todo` struct is gone.
- **service**: the business flow, written **once** for both databases. It **orchestrates the transaction** with `TxManager.WithinTx`.
- **repositories**: dumb CRUD over one table/collection, with their own row/doc structs and no transaction code. The store's `Update(id, patch)` became `GetForUpdate` + `Update` + `ActivityRepository.Create`, composed by the service.
- The transaction travels **through `context.Context`**: `WithinTx` puts it there, and repositories use it when present or the plain connection otherwise.
- Same names as before where the concept is unchanged: `Patch`, `ValidatePatch` (was `validatePatch`), `NewTodo`, `NewActivity`, `nowUTC`, `writeError`, `todoID`, `parsePage`. `applyPatch(t, p, now) []Activity` became `Todo.Apply(p, now) []Action`, because the model no longer builds rows: the service turns actions into activities.
- Sentinel errors are split by layer: `ErrNotFound`, `ErrInvalidTitle` and `ErrEmptyPatch` are domain errors in `model`, while the JSON/limit/offset errors are unexported in `handler`.
- `repotest` runs one contract suite against both adapters, including a rollback through `TxManager`.
- New tooling: zerolog (JSON request log, real error on every 500), `Taskfile.yml`, and `http.Server` with graceful shutdown on SIGINT/SIGTERM. The router uses `gin.New()` + `gin.Recovery()` + the zerolog request logger instead of `gin.Default()`, so requests aren't logged twice.

## 6. Pros / cons / when to use this in real life

**Pros**
- Easy to navigate: "where is the HTTP code / the rule / the SQL?" always has one answer.
- Repositories can be reused and combined freely, because transactions are composed in the service.
- The service is unit-testable with tiny in-memory fakes, and the handler can be tested with the real service.

**Cons**
- Packages are sliced by *technical* layer, so one feature (todos + activities) is spread over every folder. That gets painful as features grow.
- The repository interfaces live with the implementations, not with the code that uses them, so the service imports the `repository` package. Later levels flip this.
- The transaction-in-context is implicit: forgetting to pass the callback's `ctx` silently runs outside the transaction (the rollback contract test exists to catch exactly this).

**When to use it:** small to medium services with a handful of entities and one team. It is the most common Go service layout, and everyone recognizes it.

## 7. Run & test

Run these from this directory, with `docker compose up -d` running at the repo root. The API listens on `:8083`.

```sh
task run:pg          # DB_DRIVER=postgres go run ./cmd/api
task run:mongo       # DB_DRIVER=mongo go run ./cmd/api
task test            # go test ./...                     (unit: model, service with fakes, handler with httptest)
task test:integration  # go test -tags=integration ./...  (contract suite on testcontainers Postgres + Mongo rs0)
task lint            # gofmt check + go vet (both build tags)

# smoke test, from the repo root, with the app running (either driver):
python3 scripts/smoke.py http://localhost:8083
```

Gin runs in its default debug mode and prints its routes at startup. Set `GIN_MODE=release` to silence those debug lines and keep only zerolog's JSON.
