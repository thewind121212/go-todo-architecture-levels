# Level 2: flat package

## What this level teaches

The same app as level 1, with the one big `main.go` split into files by concern.
It is still a single `package main`, so this is only about navigation: you can now
find the handlers, the domain rules and each database in their own file. Nothing
else changes. Each store method still owns its own transaction and the whole
business flow inside it.

## Folder tree

```
level-02-flat/
  main.go                     env config, picks the store, runs the router
  todo.go                     Todo/Activity types, validation, sentinel errors, applyPatch
  store.go                    the store interface, the only seam in this level
  store_postgres.go           GORM implementation (AutoMigrate, one transaction per method)
  store_mongo.go              Mongo driver v2 implementation (collections + index at startup)
  handler.go                  Gin routes, handlers, writeError (the one error-to-status mapping)
  todo_test.go                table tests for validation and applyPatch
  handler_test.go             httptest handler tests against a hand-written in-memory fake store
  store_integration_test.go   one contract suite run against both stores (testcontainers)
  go.mod, go.sum
```

## Request flow: `PATCH /todos/:id`

1. **handler.go** `updateTodo`: `parseID` turns a malformed id into `ErrNotFound`,
   then `ShouldBindJSON` decodes the body into a `todoPatch` (from todo.go).
2. **store_postgres.go** or **store_mongo.go** `Update`: calls `validatePatch`
   (todo.go) to trim the title and reject an empty patch, then opens a transaction.
3. Inside the transaction the store reads the todo, calls `applyPatch` (todo.go) to
   change it and get one `Activity` per real change, saves the todo and inserts the
   activities. Any error rolls the whole thing back.
4. **handler.go**: on success it writes `200` with the todo. On error, `writeError`
   maps `ErrNotFound` to 404, validation errors to 400 and anything else to 500.

## Where the transaction lives

Inside each store method, exactly as in level 1. Each store has a small `inTx`
helper so every transaction starts in one place:

```go
// store_postgres.go
func (s *postgresStore) inTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

func (s *postgresStore) Update(ctx context.Context, id string, p todoPatch) (Todo, error) {
	// ... validatePatch ...
	err = s.inTx(ctx, func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&todo, "id = ?", id).Error
		// ... ErrNotFound / wrap ...
		activities := applyPatch(&todo, p, now())
		if len(activities) == 0 {
			return nil
		}
		if err := tx.Save(&todo).Error; err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if err := tx.Create(&activities).Error; err != nil {
			return fmt.Errorf("insert activities: %w", err)
		}
		return nil
	})
	// ...
}
```

Mongo does the same with `session.WithTransaction`. Inside `fn`, every collection call
uses the `ctx` that `WithTransaction` passes in, because that ctx carries the session:

```go
// store_mongo.go
func (s *mongoStore) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}
```

The rollback test in `store_integration_test.go` uses these same helpers: it writes a
todo and an activity inside `inTx`, returns an error, and checks that nothing was saved.

## What changed from the previous level

Level 1 had all of this in one `main.go`. Level 2 moves the same code into six files,
and the tests sit next to the code they test. The behaviour, the store interface and
the place of the transaction are unchanged. The rules shared by both stores (`newTodo`,
`validatePatch`, `applyPatch`) live in `todo.go`, so it is easy to see that both stores
call them.

## Pros / cons / when to use this in real life

**Pros**
- Easy to navigate: the file name tells you where to look.
- Still nothing to learn. No packages, no imports between your own files, no layers.
- Smaller diffs and fewer merge conflicts than one big file.

**Cons**
- It is still one package, so the file boundaries are only a convention. Nothing
  stops `handler.go` from calling GORM directly, or a store from writing HTTP responses.
- The store still owns the business flow. Both stores repeat the same steps (validate,
  read, `applyPatch`, save, insert activities), and the fake store in the handler tests
  repeats them a third time. A new rule means editing every store.
- You cannot test the business flow without a database or a fake that copies it.
- The domain types carry `json`, `gorm` and `bson` tags, so storage details leak
  into the API types and the other way round.

**When to use it**: small services, CLIs and prototypes with one or two people, where
the whole app fits in your head. When the store methods start to repeat business
logic, move that logic out of the store. That is level 3.

## Run & test

From this directory, with the compose databases running (`docker compose up -d` at repo root):

```sh
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH

# run (http://localhost:8082)
DB_DRIVER=postgres go run .
DB_DRIVER=mongo go run .

# unit tests (no Docker)
go test ./...

# integration tests (start their own Postgres and Mongo containers)
go test -tags=integration ./...

# smoke test, from repo root, while the app is running
python3 scripts/smoke.py http://localhost:8082
```

Config: `DB_DRIVER` (`postgres` or `mongo`), `POSTGRES_DSN`, `MONGO_URI`, `MONGO_DB`
(default `todo_l02`) and `HTTP_ADDR` (default `:8082`).
