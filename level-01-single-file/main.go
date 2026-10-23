// Level 1: the whole Todo app in one file. Read it top to bottom:
// domain, then the two storage adapters, then HTTP, then main.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// --- domain ---

// Todo carries JSON, GORM and BSON tags at once: one type serves every layer.
// The created/updated times are set by the app, so GORM's auto-timestamps are off.
type Todo struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	Title     string    `json:"title" gorm:"not null" bson:"title"`
	Completed bool      `json:"completed" gorm:"not null;default:false" bson:"completed"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;autoUpdateTime:false" bson:"updated_at"`
}

// Activity is one entry in a todo's append-only audit log. It has no foreign
// key on purpose: activities outlive the todo they describe.
type Activity struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	TodoID    string    `json:"todo_id" gorm:"type:uuid;not null;index" bson:"todo_id"`
	Action    Action    `json:"action" gorm:"not null" bson:"action"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
}

type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Patch is a partial update: a nil field means "leave it alone".
type Patch struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

var (
	ErrNotFound      = errors.New("todo not found")
	ErrInvalidTitle  = errors.New("title must be 1 to 200 characters")
	ErrEmptyPatch    = errors.New("patch must set title or completed")
	ErrInvalidLimit  = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset = errors.New("offset must be 0 or more")
	ErrInvalidJSON   = errors.New("invalid JSON body")
)

const maxTitleLen = 200

// nowUTC is truncated to milliseconds because Mongo stores milliseconds;
// this way both stores round-trip exactly the same value.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// newID returns a time-ordered UUIDv7, so sorting by id sorts by creation time.
// NewV7 only fails if crypto/rand fails, and since Go 1.24 that crashes the
// program instead of returning an error, so Must can never panic here.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func validateTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

func newTodo(title string) (Todo, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	now := nowUTC()
	return Todo{ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// validatePatch trims the title and rejects a patch that sets nothing.
func validatePatch(p Patch) (Patch, error) {
	if p.Title == nil && p.Completed == nil {
		return Patch{}, ErrEmptyPatch
	}
	if p.Title != nil {
		title, err := validateTitle(*p.Title)
		if err != nil {
			return Patch{}, err
		}
		p.Title = &title
	}
	return p, nil
}

func newActivity(todoID string, action Action, at time.Time) Activity {
	return Activity{ID: newID(), TodoID: todoID, Action: action, CreatedAt: at}
}

// applyPatch changes t in place and returns one activity per real change
// (none if nothing changed). Both stores call it inside their transaction,
// so the "what changed -> which activities" rule is written only once.
func applyPatch(t *Todo, p Patch, now time.Time) []Activity {
	var activities []Activity
	if p.Title != nil && *p.Title != t.Title {
		t.Title = *p.Title
		activities = append(activities, newActivity(t.ID, ActionRenamed, now))
	}
	if p.Completed != nil && *p.Completed != t.Completed {
		t.Completed = *p.Completed
		action := ActionReopened
		if t.Completed {
			action = ActionCompleted
		}
		activities = append(activities, newActivity(t.ID, action, now))
	}
	if len(activities) > 0 {
		t.UpdatedAt = now
	}
	return activities
}

// store is everything the HTTP handlers need from a database. Postgres and
// Mongo both implement it, and main picks one at startup.
type store interface {
	Create(ctx context.Context, t Todo) error
	Get(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, limit, offset int) ([]Todo, error)
	Update(ctx context.Context, id string, p Patch) (Todo, error)
	Delete(ctx context.Context, id string) error
	ListActivities(ctx context.Context, todoID string) ([]Activity, error)
	Ping(ctx context.Context) error
}

// --- storage: postgres ---

type postgresStore struct {
	db *gorm.DB
}

func newPostgresStore(ctx context.Context, dsn string) (*postgresStore, error) {
	// Silent: errors are returned to the caller, and the HTTP layer logs the ones that matter.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&Todo{}, &Activity{}); err != nil {
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return &postgresStore{db: db}, nil
}

// inTx runs fn in one database transaction: commit if fn returns nil, roll back otherwise.
func (s *postgresStore) inTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

func (s *postgresStore) Create(ctx context.Context, t Todo) error {
	return s.inTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		a := newActivity(t.ID, ActionCreated, t.CreatedAt)
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *postgresStore) Get(ctx context.Context, id string) (Todo, error) {
	return findTodo(s.db.WithContext(ctx), id)
}

func (s *postgresStore) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	todos := []Todo{}
	err := s.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&todos).Error
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	for i := range todos {
		todos[i] = todoInUTC(todos[i])
	}
	return todos, nil
}

func (s *postgresStore) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	var t Todo
	err := s.inTx(ctx, func(tx *gorm.DB) error {
		// FOR UPDATE locks the row until commit, so two concurrent PATCHes
		// can't both see "not completed" and both log a "completed" activity.
		var err error
		t, err = findTodo(tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
		if err != nil {
			return err
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
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return t, nil
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&Todo{})
		if res.Error != nil {
			return fmt.Errorf("delete todo: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		a := newActivity(id, ActionDeleted, nowUTC())
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *postgresStore) ListActivities(ctx context.Context, todoID string) ([]Activity, error) {
	activities := []Activity{}
	err := s.db.WithContext(ctx).Where("todo_id = ?", todoID).Order("id ASC").Find(&activities).Error
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	for i := range activities {
		activities[i].CreatedAt = activities[i].CreatedAt.UTC()
	}
	return activities, nil
}

func (s *postgresStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// findTodo works on a plain or a transactional *gorm.DB, so Get and Update share it.
func findTodo(db *gorm.DB, id string) (Todo, error) {
	var t Todo
	err := db.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return todoInUTC(t), nil
}

// todoInUTC is needed because pgx returns timestamps in the process's local
// time zone, while the API promises UTC.
func todoInUTC(t Todo) Todo {
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t
}

// --- storage: mongo ---

type mongoStore struct {
	client     *mongo.Client
	todos      *mongo.Collection
	activities *mongo.Collection
}

func newMongoStore(ctx context.Context, uri, dbName string) (*mongoStore, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	db := client.Database(dbName)
	// Collections are created up front because creating one inside a
	// multi-document transaction is not supported on every Mongo version.
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	for _, name := range []string{"todos", "activities"} {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return nil, fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	activities := db.Collection("activities")
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := activities.Indexes().CreateOne(ctx, index); err != nil {
		return nil, fmt.Errorf("create activities index: %w", err)
	}
	return &mongoStore{client: client, todos: db.Collection("todos"), activities: activities}, nil
}

// inTx runs fn in one multi-document transaction. fn must pass the ctx it is
// given to every collection call: that ctx carries the session, and the
// session is what puts the call inside the transaction.
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

func (s *mongoStore) Create(ctx context.Context, t Todo) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if _, err := s.todos.InsertOne(ctx, t); err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		if _, err := s.activities.InsertOne(ctx, newActivity(t.ID, ActionCreated, t.CreatedAt)); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *mongoStore) Get(ctx context.Context, id string) (Todo, error) {
	var t Todo
	err := s.todos.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return t, nil
}

func (s *mongoStore) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := s.todos.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	todos := []Todo{}
	if err := cursor.All(ctx, &todos); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	return todos, nil
}

func (s *mongoStore) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	var t Todo
	// If another transaction changes this todo first, Mongo reports a write
	// conflict and WithTransaction reruns fn, so a stale read is never committed.
	err := s.inTx(ctx, func(ctx context.Context) error {
		var err error
		t, err = s.Get(ctx, id)
		if err != nil {
			return err
		}
		activities := applyPatch(&t, p, nowUTC())
		if len(activities) == 0 {
			return nil
		}
		if _, err := s.todos.ReplaceOne(ctx, bson.D{{Key: "_id", Value: t.ID}}, t); err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if _, err := s.activities.InsertMany(ctx, activities); err != nil {
			return fmt.Errorf("insert activities: %w", err)
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return t, nil
}

func (s *mongoStore) Delete(ctx context.Context, id string) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		res, err := s.todos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
		if err != nil {
			return fmt.Errorf("delete todo: %w", err)
		}
		if res.DeletedCount == 0 {
			return ErrNotFound
		}
		if _, err := s.activities.InsertOne(ctx, newActivity(id, ActionDeleted, nowUTC())); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *mongoStore) ListActivities(ctx context.Context, todoID string) ([]Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := s.activities.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := []Activity{}
	if err := cursor.All(ctx, &activities); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	return activities, nil
}

func (s *mongoStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}

// --- http ---

type handler struct {
	store store
}

func newRouter(s store) *gin.Engine {
	h := handler{store: s}
	r := gin.Default()
	r.GET("/health", h.health)
	r.POST("/todos", h.createTodo)
	r.GET("/todos", h.listTodos)
	r.GET("/todos/:id", h.getTodo)
	r.PATCH("/todos/:id", h.updateTodo)
	r.DELETE("/todos/:id", h.deleteTodo)
	r.GET("/todos/:id/activities", h.listActivities)
	return r
}

func (h handler) health(c *gin.Context) {
	if err := h.store.Ping(c.Request.Context()); err != nil {
		writeError(c, fmt.Errorf("ping: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h handler) createTodo(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, ErrInvalidJSON)
		return
	}
	t, err := newTodo(req.Title)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.store.Create(c.Request.Context(), t); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h handler) listTodos(c *gin.Context) {
	limit, offset, err := parsePage(c)
	if err != nil {
		writeError(c, err)
		return
	}
	todos, err := h.store.List(c.Request.Context(), limit, offset)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": todos, "limit": limit, "offset": offset})
}

func (h handler) getTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	t, err := h.store.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h handler) updateTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var p Patch
	if err := c.ShouldBindJSON(&p); err != nil {
		writeError(c, ErrInvalidJSON)
		return
	}
	p, err = validatePatch(p)
	if err != nil {
		writeError(c, err)
		return
	}
	t, err := h.store.Update(c.Request.Context(), id, p)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h handler) deleteTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h handler) listActivities(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	activities, err := h.store.ListActivities(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": activities})
}

// todoID reads the :id path parameter. A malformed id can't match any todo,
// so it is reported as not found rather than as a bad request.
func todoID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", ErrNotFound
	}
	return id.String(), nil
}

func parsePage(c *gin.Context) (limit, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, 0, ErrInvalidLimit
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, ErrInvalidOffset
	}
	return limit, offset, nil
}

// writeError is the one place that turns an error into an HTTP status.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrInvalidTitle),
		errors.Is(err, ErrEmptyPatch),
		errors.Is(err, ErrInvalidLimit),
		errors.Is(err, ErrInvalidOffset):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// --- main ---

func main() {
	ctx := context.Background()
	s, err := openStore(ctx, getenv("DB_DRIVER", "postgres"))
	if err != nil {
		log.Fatal(err)
	}
	if err := newRouter(s).Run(getenv("HTTP_ADDR", ":8081")); err != nil {
		log.Fatal(err)
	}
}

// openStore is the manual dependency injection: config picks the store,
// and everything else only sees the store interface.
func openStore(ctx context.Context, driver string) (store, error) {
	switch driver {
	case "postgres":
		dsn := getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l01?sslmode=disable")
		s, err := newPostgresStore(ctx, dsn)
		if err != nil {
			// Return a literal nil: a nil *postgresStore wrapped in the store
			// interface would not compare equal to nil.
			return nil, err
		}
		return s, nil
	case "mongo":
		uri := getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true")
		s, err := newMongoStore(ctx, uri, getenv("MONGO_DB", "todo_l01"))
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown DB_DRIVER %q: want postgres or mongo", driver)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
