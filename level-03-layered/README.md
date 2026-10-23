# Level 3: classic layered (handler → service → repository)

## 1. What this level teaches

The code is split into three layers, and each one has a single job. The **handler** speaks HTTP, the **service** holds the business flow, and the **repositories** do plain single-table CRUD. Most importantly, the transaction moves out of storage and into the service: `TxManager.WithinTx` puts the transaction into the `context.Context`, and the repositories pick it up from there. The level also adds the first bits of production tooling: zerolog, a Taskfile and graceful shutdown.

## 2. Folder tree

```
level-03-layered/
  cmd/api/main.go                     config from env, open DB, wire layers by hand, http.Server + graceful shutdown
  internal/
    model/                            plain Todo/Activity types, sentinel errors, title rule, Todo.Apply (which activities a patch produces)
    repository/
      repository.go                   the contracts: TodoRepository, ActivityRepository, TxManager
      postgres/                       GORM adapter: AutoMigrate, TxManager (tx in ctx), one repo per table
      mongo/                          driver v2 adapter: collections + index at startup, TxManager (session in ctx), one repo per collection
      repotest/                       contract suite both adapters run in their integration tests
      fake/                           hand-written in-memory repos + fake TxManager for unit tests
    service/todo_service.go           TodoService: business flows, opens the transactions
    handler/                          Gin routes, request/response DTOs, request logger, writeError (the one error→status map)
  Taskfile.yml                        run:pg, run:mongo, test, test:integration, lint
```

## 3. Request flow: `PATCH /todos/:id`

1. `handler/router.go`: Gin matches the route; `requestLogger` will log method, path, status and latency when the request finishes.
2. `handler/todo_handler.go` `updateTodo`: binds the JSON into `updateTodoRequest` (pointer fields, so a missing field is `nil`), turns it into a `model.Patch` and calls `TodoService.Update`.
3. `service/todo_service.go` `Update`: rejects a malformed id as `ErrNotFound`, then calls `tx.WithinTx`. Inside the callback, using the callback's `ctx`, it:
   - reads the todo with `todos.Get`,
   - calls `todo.Apply(patch, now)` from `model/todo.go`, which validates the title and returns the actions (`renamed`, `completed`, `reopened`) for the fields that really changed,
   - saves with `todos.Update` and inserts one activity per action with `activities.Create`.
4. `repository/postgres/*_repository.go` (or `repository/mongo/...`): each method runs its single statement. In Postgres `conn(ctx, db)` finds the GORM tx in `ctx`. In Mongo the `ctx` itself already carries the session.
5. Back in the service, `WithinTx` commits if the callback returned `nil` and rolls back otherwise.
6. `handler/todo_handler.go`: on success, maps `model.Todo` to `todoResponse` (`dto.go`) and returns 200. On error, `handler/errors.go` `writeError` maps it to 400/404/500 and logs any 500 with the real error.

## 4. Where the transaction lives

The **service** decides what belongs in one transaction (`internal/service/todo_service.go`):

```go
err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
    todo, err = s.todos.Get(ctx, id)
    ...
    actions, err := todo.Apply(patch, now)
    ...
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

Mongo needs no `ctxKey` at all. Driver v2 already hands the callback a `ctx` that carries the session, and any collection call made with that `ctx` joins the transaction (`internal/repository/mongo/db.go`):

```go
_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
    return nil, fn(ctx)
})
```

That is why the Mongo repositories just use the `ctx` they get, while the Postgres ones call `conn(ctx, r.db)`. Note that `WithTransaction` may re-run `fn` on transient errors, so `fn` must only touch the database.

## 5. What changed from level 2

- In level 2 one store type owned the whole flow: each store method validated, read, wrote the todo, wrote the activities and opened and committed its own transaction. Now every layer has one job:
  - the handler does HTTP only, with its own DTOs, so JSON tags never reach the model,
  - the service owns the business flow and **orchestrates the transaction**,
  - repositories are dumb CRUD over one table/collection, with no transaction code of their own.
- The transaction travels **through `context.Context`**: `TxManager.WithinTx` puts it there, and repositories use it when it is present or the plain connection when it is not.
- `repotest` runs one contract suite against both adapters, including a rollback through `TxManager`.
- New tooling: zerolog (JSON request log plus the real error on every 500), `Taskfile.yml`, and `http.Server` with graceful shutdown on SIGINT/SIGTERM.

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

Gin starts in debug mode and prints its routes. Set `GIN_MODE=release` to get only zerolog's JSON lines.
