# Todo Levels: one app, nine architectures

The same deliberately tiny Todo API, built nine times in Go. Each level uses a more advanced architecture,
from a single `main.go` up to a modular monolith. **The behavior never changes, only the structure does**,
so you can compare the levels and see exactly what each architecture buys you and what it costs.

Every level has:

* **two storage adapters, Postgres and MongoDB, behind an interface.** You pick one at startup with `DB_DRIVER`.
* **real transactions in both databases.** Each change to a todo and its activity-log entry are written
  atomically (Mongo runs as a replica set, which Mongo transactions require).
* the same HTTP API, the same black-box smoke test, unit tests, and integration tests that run one contract
  suite against both databases (including a rollback test).

## The levels

| # | Folder | Architecture | The one new idea | Where the transaction lives | Tools that appear |
|---|---|---|---|---|---|
| 1 | [`level-01-single-file`](level-01-single-file) | everything in `main.go` | it works, but read the "cons" | inside each store method | Gin, GORM, mongo-driver |
| 2 | [`level-02-flat`](level-02-flat) | level 1 split into files | files are not boundaries | inside each store method | |
| 3 | [`level-03-layered`](level-03-layered) | handler → service → repository | tx moves to the service, carried in `ctx` | service, via `TxManager.WithinTx` | zerolog, Taskfile, graceful shutdown |
| 4 | [`level-04-by-feature`](level-04-by-feature) | package by feature | cohesion + consumer-side interfaces | todo service; one tx spans two features | |
| 5 | [`level-05-clean`](level-05-clean) | Clean Architecture | the dependency rule, enforced by a test | one use case (interactor) per struct | config package, arch test |
| 6 | [`level-06-hexagonal`](level-06-hexagonal) | Ports & Adapters ([go-scaffolding](https://github.com/cuongtranba/go-scaffolding) style) | named driving/driven ports | service, via `ports.TxManager` | **Wire**, **mockery**, **SQL migrations** |
| 7 | [`level-07-ddd`](level-07-ddd) | DDD inside the hexagon | aggregate, value objects, domain events | `repository.Save` (aggregate = tx boundary) | optimistic locking (409) |
| 8 | [`level-08-cqrs`](level-08-cqrs) | DDD + CQRS + outbox | separate write and read models | Save (state + outbox), relay (projection + mark) | background relay |
| 9 | [`level-09-modular-monolith`](level-09-modular-monolith) | modular monolith | modules own their data, talk via events | per module; across modules via outbox | in-process event bus |

Tools show up only at the level that needs them: manual DI until level 6, hand-written fakes until mockery,
`AutoMigrate` until real migrations. Each level's README says why a tool appears where it does.

## Quick start

```sh
# 1. Go toolchain (installed in ~/.local/go)
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH

# 2. Databases shared by all levels: Postgres 17 + Mongo 8 (replica set rs0)
docker compose up -d

# 3. Run any level, with either database (level N listens on :808N)
cd level-03-layered
DB_DRIVER=postgres go run ./cmd/api      # levels 1–2: go run .
DB_DRIVER=mongo    go run ./cmd/api

# 4. From the repo root, run the black-box smoke test against it
python3 scripts/smoke.py http://localhost:8083
```

Tests, in any level:

```sh
go test ./...                      # unit tests, no Docker needed
go test -tags=integration ./...    # starts its own Postgres + Mongo containers (testcontainers)
```

Each level is an independent Go module, so you can copy one out as the starting point for a new project.

## How to study this repo

1. Read the levels **in order**. Each README opens with one architecture diagram, then has the same sections:
   what it teaches, the tree, how `PATCH /todos/:id` flows through the code, where the transaction lives, what
   changed, and pros/cons/when to use it.
2. **Diff consecutive levels.** That is the point of the repo: the diff should show the new idea and little else.
   ```sh
   git diff --no-index level-02-flat level-03-layered                   # flat → layers
   git diff --no-index level-03-layered level-04-by-feature             # layers → features
   git diff --no-index level-06-hexagonal level-07-ddd                  # hexagon → DDD
   ```
3. Follow the transaction. Find the same rule in every level ("a change and its activity are written
   atomically") and watch where it moves: store → service → use case → repository → outbox.

## API (identical in every level)

| Method & path | Success |
|---|---|
| `GET /health` | 200 `{"status":"ok"}` |
| `POST /todos` `{"title"}` | 201 todo |
| `GET /todos?limit=&offset=` | 200 `{"items":[...],"limit":20,"offset":0}` |
| `GET /todos/:id` | 200 todo |
| `PATCH /todos/:id` `{"title"?, "completed"?}` | 200 todo |
| `DELETE /todos/:id` | 204 |
| `GET /todos/:id/activities` | 200 `{"items":[...]}` (created, renamed, completed, reopened, deleted) |

Errors are always `{"error":"..."}`: 400 for bad input, 404 for an unknown or malformed id, 409 for a
concurrent update (levels 7–9), 500 otherwise. The full contract is in [SPEC.md](SPEC.md).

## Which one should I use for real?

* **A script, a spike, a tiny service:** level 1–2. Don't apologize for it.
* **A typical CRUD service with one team:** level 3 or 4. Layered is the most familiar; by-feature scales better.
* **A long-lived service where the business core should outlive frameworks:** level 5 or 6.
* **A domain with real rules and invariants (orders, billing, scheduling):** level 7.
* **Reads and writes with very different needs, or other systems consuming your events:** level 8.
* **Several teams in one codebase, with a possible later split into services:** level 9.

The Todo app needs none of levels 5–9. That is on purpose: the domain is trivial, so all you see is the architecture.

## Repo layout

```
SPEC.md                  the contract every level follows (API, domain rules, conventions)
docker-compose.yml       Postgres + Mongo replica set, one database per level (todo_l01 … todo_l09)
deploy/postgres-init.sql creates those databases
scripts/smoke.py         black-box API test, run against every level and both databases
level-0N-*/              the nine levels, each its own Go module
```
