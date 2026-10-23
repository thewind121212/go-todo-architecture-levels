# Level 5: Clean Architecture

## What this level teaches

The **Dependency Rule** from Uncle Bob's Clean Architecture: source code dependencies point only inward,
so the business core (`entity` and `usecase`) knows nothing about Gin, GORM, Mongo or zerolog. The core
declares the interfaces it needs (`usecase/ports.go`) and the adapters implement them. Each use case is
its own small struct with one `Execute(ctx, Input)` method. A test (`arch_test.go`) fails the build if
the core ever imports an outer layer.

```
        +-----------------------------------------------------+
        |  infrastructure + cmd/api   (config, server, main)  |
        |   +---------------------------------------------+   |
        |   |  adapter   (http, postgres, mongo)          |   |
        |   |   +-------------------------------------+   |   |
        |   |   |  usecase   (interactors + ports)    |   |   |
        |   |   |   +-----------------------------+   |   |   |
        |   |   |   |  entity  (Todo, Activity,   |   |   |   |
        |   |   |   |   rules, sentinel errors)   |   |   |   |
        |   |   |   +-----------------------------+   |   |   |
        |   |   +-------------------------------------+   |   |
        |   +---------------------------------------------+   |
        +-----------------------------------------------------+
                 every import arrow points inward  --->
```

## Folder tree

```
level-05-clean/
  arch_test.go                       the Dependency Rule as a test (go/parser, stdlib only)
  Taskfile.yml                       run:pg, run:mongo, test, test:integration, lint
  cmd/api/main.go                    composition root: builds every concrete type by hand and wires them
  internal/
    entity/                          innermost circle; imports only the stdlib and uuid
      todo.go                        Todo, NewTodo, ValidateTitle, Rename, SetCompleted
      activity.go                    Activity, NewActivity and the Action values
      errors.go                      ErrNotFound, ErrInvalidTitle, ErrEmptyPatch
    usecase/                         application rules; imports only entity
      ports.go                       TodoRepository, ActivityRepository, TxManager (what the core needs)
      now.go                         nowUTC, taken once per Execute
      output.go                      TodoOutput, ActivityOutput (plain data handed outward)
      create_todo.go ... list_activities.go   one interactor per file, each with its Input/Output
      fakes_test.go                  hand-written in-memory fakes of the ports
    adapter/
      http/                          Gin: controllers + input ports, request parsing (id, limit/offset),
                                     presenters, writeError, router
      postgres/                      GORM models + mappers, repositories, TxManager, AutoMigrate
      mongo/                         BSON documents + mappers, repositories, TxManager, collections/index
      contract_test.go               shared integration suite every adapter must pass (incl. rollback)
      adapters_test.go               runs the suite on Postgres and Mongo in testcontainers
    infrastructure/
      config/                        env vars -> Config, fails fast on an unknown DB_DRIVER
      server/                        http.Server with graceful shutdown
```

## Request flow: `PATCH /todos/:id`

1. `adapter/http/router.go` routes the request to `TodoController.Update`.
2. `adapter/http/todo_controller.go` binds the JSON into `updateTodoRequest` (pointer fields, so a missing
   field is `nil`), and `todoID` in `adapter/http/request.go` turns `:id` into a canonical UUID (malformed
   is `ErrNotFound`, so 404). It then calls `UpdateTodoUseCase.Execute` with a `usecase.UpdateTodoInput`.
   The controller only knows that one-method interface, not the concrete interactor.
3. `usecase/update_todo.go` validates before touching the database: an empty patch is `ErrEmptyPatch` and
   a bad title is `ErrInvalidTitle` (via `entity.ValidateTitle`), so both are 400 even for an unknown id.
   It takes `now := nowUTC()` once and opens a transaction with `tx.WithinTx`.
4. Inside the transaction: `todos.GetForUpdate` (Postgres: `SELECT ... FOR UPDATE`), then `applyChanges`
   calls the entity's methods.
5. `entity/todo.go`: `Rename(title, now)` and `SetCompleted(completed, now)` change the todo and report
   whether something really changed (`SetCompleted` also says `completed` or `reopened`).
6. Back in the interactor: if nothing changed, it is done. Otherwise `todos.Update` and one
   `activities.Create` per action, each with the same `now`, all inside the same transaction.
7. `adapter/postgres/todo_repository.go` (or `adapter/mongo/...`) maps the entity to its row/document and
   writes it, using the transaction found in `ctx`.
8. The interactor returns a `usecase.TodoOutput`; `adapter/http/presenter.go` maps it to the JSON shape.
   Any error goes to `writeError` in `adapter/http/errors.go`, the one place that picks a status code.

## Where the transaction lives

In the **use case interactor**. It decides what belongs together; the adapters only know how to run a
transaction (`usecase/update_todo.go`):

```go
now := nowUTC()
var todo entity.Todo
// Inside the transaction only the diff happens: read the current todo (locked), work out what
// changed, and write the todo with one activity per change.
err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
	var err error
	todo, err = uc.todos.GetForUpdate(ctx, in.ID)
	if err != nil {
		return err
	}
	actions, err := applyChanges(&todo, in, now)
	...
	if err := uc.todos.Update(ctx, todo); err != nil {
		return err
	}
	for _, action := range actions {
		activity, err := entity.NewActivity(todo.ID, action, now)
		...
		if err := uc.activities.Create(ctx, activity); err != nil {
			return err
		}
	}
	return nil
})
```

`TxManager` is a port declared in `usecase/ports.go`. The Postgres adapter puts the `*gorm.DB` transaction
into `ctx` and its repositories pick it up with `conn(ctx, db)`. The Mongo adapter uses
`session.WithTransaction`, whose callback `ctx` already carries the session. Mongo's `GetForUpdate` is a
plain find: a concurrent writer causes a write conflict and `WithTransaction` retries.

## What changed from the previous level

Level 4 sliced the code by feature; level 5 slices it by **circle**, and makes the direction of the
imports a rule:

```
level 4                                   level 5
internal/todo/      todo.go service.go     internal/entity/     todo.go activity.go errors.go
                    handler.go postgres.go internal/usecase/    ports.go + one file per use case
                    mongo.go repository.go internal/adapter/    http/ postgres/ mongo/
internal/activity/  (same shape)           internal/infrastructure/  config/ server/
internal/platform/  database/ tx/ httpx/   arch_test.go
```

* **The Dependency Rule is enforced by a test.** In level 4 nothing stopped `todo/service.go` from importing
  GORM; it just didn't. Here `arch_test.go` parses the imports of `internal/entity` and `internal/usecase`
  and fails on anything except the standard library, uuid and the inner circles. Gin, GORM, Mongo and
  zerolog can only appear in `adapter`, `infrastructure` and `cmd`.
* **One use case per struct.** Level 4's `todo.Service` with `Create/Get/List/Update/Delete` methods
  becomes `CreateTodo`, `GetTodo`, `ListTodos`, `UpdateTodo`, `DeleteTodo` and `ListActivities`, each with
  `Execute(ctx, Input) (Output, error)` (`DeleteTodo` returns only an error). Level 4's `Patch` becomes
  `UpdateTodoInput`: the use case's own input boundary.
* **Controllers depend on input ports.** Level 4's `Handler` held a `*Service`; here `TodoController` holds
  one-method interfaces (`CreateTodoUseCase`, ...) declared next to it, so its tests use tiny func fakes.
  Presenters map `TodoOutput` to JSON.
* **Rich entity methods.** Level 4's single `Todo.Apply(Patch)` is split into `Rename(title, now)` and
  `SetCompleted(completed, now)`; the use case decides which to call, the entity guards its own rules.
  The constructor is `entity.NewTodo` (not `todo.New`), because package `entity` holds more than one type.
* **No cross-feature interface.** Level 4's `ActivityRecorder` existed because todo must not import the
  activity feature. With layers there is no such boundary: the interactors use `ActivityRepository` directly.
* **New: a `config` package** in `infrastructure/config`, replacing level 4's `loadConfig` in `main.go`,
  and `infrastructure/server` for the graceful-shutdown `http.Server`.
* Unchanged on purpose: tag-free entities with row/doc mappers in each adapter (since level 3),
  `TxManager.WithinTx`, `GetForUpdate`, `ListByTodo`, `nowUTC`, `writeError`.
* DI is still **manual**: `cmd/api/main.go` reads top to bottom in one screen, so a generator (Wire, level 6)
  would not pay for itself yet.

## Pros / cons / when to use this in real life

Pros
* The business rules can be read and tested without any framework, database or Docker.
* Swapping a database or the web framework touches only an adapter and `main.go`; the contract suite
  shows that the new adapter behaves the same.
* One file per use case makes it obvious what the app does and keeps each change small.
* The Dependency Rule is checked by a test, not just by convention.

Cons
* More types and mapping code: entity, persistence model, use case output and JSON response for one todo,
  plus an input port per use case.
* More files and indirection for a CRUD app this small; a newcomer has to learn the circles first.
* Layer-first folders spread one feature over four packages (level 4's by-feature slicing helps there).

When to use it: services with real business rules that should outlive their frameworks, apps that need
more than one delivery mechanism or database, and teams that want boundaries checked automatically.
For a thin CRUD service, level 3 or 4 is usually enough.

## Run & test

```sh
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH
docker compose up -d                 # at the repo root, once

task run:pg                          # or: DB_DRIVER=postgres go run ./cmd/api   (listens on :8085)
                                     # GIN_MODE=release silences Gin's debug lines
task run:mongo                       # or: DB_DRIVER=mongo go run ./cmd/api

task test                            # go test ./...                      unit + architecture tests
task test:integration                # go test -tags=integration ./...   both adapters in testcontainers
task lint                            # gofmt -l, go vet (both build tags)

# smoke test, with the app running (from the repo root):
python3 scripts/smoke.py http://localhost:8085
```
