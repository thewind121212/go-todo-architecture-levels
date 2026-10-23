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
      todo.go                        Todo, title rules, Rename, SetCompleted
      activity.go                    Activity and its Action values
      errors.go                      ErrNotFound, ErrInvalidTitle
    usecase/                         application rules; imports only entity
      ports.go                       TodoRepository, ActivityRepository, TxManager (what the core needs)
      errors.go                      ErrInvalidPagination, ErrEmptyUpdate
      output.go                      TodoOutput, ActivityOutput (plain data handed outward)
      create_todo.go ... list_activities.go   one interactor per file, each with its Input/Output
      fakes_test.go                  hand-written in-memory fakes of the ports
    adapter/
      http/                          Gin: controllers, input-port interfaces, presenters, writeError, router
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
   field is `nil`) and calls `UpdateTodoUseCase.Execute` with a `usecase.UpdateTodoInput`. The controller
   only knows that one-method interface, not the concrete interactor.
3. `usecase/update_todo.go` rejects an empty input (`ErrEmptyUpdate`), then opens a transaction with
   `tx.WithinTx`. Inside it: `todos.FindByID`, then `applyChanges` calls the entity.
4. `entity/todo.go`: `Rename` validates and trims the title, `SetCompleted` returns `completed` or
   `reopened`. Each reports whether something really changed.
5. Back in the interactor: if nothing changed, it is done. Otherwise `todos.Update` and one
   `activities.Create` per action, all inside the same transaction.
6. `adapter/postgres/todo_repository.go` (or `adapter/mongo/...`) maps the entity to its persistence model
   and writes it, using the transaction found in `ctx`.
7. The interactor returns a `usecase.TodoOutput`; `adapter/http/presenter.go` maps it to the JSON shape.
   Any error goes to `writeError` in `adapter/http/errors.go`, the one place that picks a status code.

## Where the transaction lives

In the **use case interactor**. It decides what belongs together; the adapters only know how to do a
transaction (`usecase/update_todo.go`):

```go
var todo entity.Todo
// The read is inside the transaction too, because the changes are computed from it.
err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
	var err error
	todo, err = uc.todos.FindByID(ctx, in.ID)
	if err != nil {
		return err
	}
	actions, err := applyChanges(&todo, in)
	...
	if err := uc.todos.Update(ctx, todo); err != nil {
		return err
	}
	for _, action := range actions {
		if err := uc.activities.Create(ctx, entity.NewActivity(todo.ID, action)); err != nil {
			return err
		}
	}
	return nil
})
```

`TxManager` is a port declared in `usecase/ports.go`. The Postgres adapter puts the `*gorm.DB` transaction
into `ctx` and its repositories pick it up with `conn(ctx, db)`. The Mongo adapter uses
`session.WithTransaction`, whose callback `ctx` already carries the session.

The second lesson is **mapping at the boundary**. Entities have no tags; each adapter owns its own model:

```go
// internal/entity/todo.go: pure business data
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// internal/adapter/postgres/models.go: storage details live here
type todoModel struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Title     string    `gorm:"not null"`
	...
}

func toTodoEntity(m todoModel) entity.Todo {
	return entity.Todo{ID: m.ID, Title: m.Title, Completed: m.Completed,
		CreatedAt: m.CreatedAt.UTC(), UpdatedAt: m.UpdatedAt.UTC()} // pgx returns local time
}
```

`adapter/mongo/documents.go` does the same with `bson` tags, and `adapter/http/presenter.go` with `json` tags.

## What changed from the previous level

* Compared to L3/L4, **the business core now knows nothing about Gin, GORM or Mongo, and a test enforces it.**
  `arch_test.go` parses the imports of `internal/entity` and `internal/usecase` and fails on anything other
  than the standard library, uuid and the inner circles.
* The service is split into **one interactor per use case** (`CreateTodo`, `UpdateTodo`, ...), each with
  its own `Input` and `Output` types.
* **Interfaces moved to the consumer.** Repository and `TxManager` ports live in `usecase/ports.go`; the
  input ports the controllers call live next to the controllers.
* **Entities lost their tags.** GORM models and BSON documents are separate structs with mappers.
* **New: a `config` package** and an `infrastructure` circle for config and the HTTP server.
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
* More types and mapping code: entity, persistence model, use case output and JSON response for one todo.
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
task run:mongo                       # or: DB_DRIVER=mongo go run ./cmd/api

task test                            # go test ./...                      unit + architecture tests
task test:integration                # go test -tags=integration ./...   both adapters in testcontainers
task lint                            # gofmt -l, go vet (both build tags)

# smoke test, with the app running (from the repo root):
python3 scripts/smoke.py http://localhost:8085
```
