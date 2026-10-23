package http

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"example.com/todo/level05/internal/entity"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// parseID returns the canonical form of the :id path parameter. A malformed id can never match a
// todo, so it is reported as not found before any use case runs.
func parseID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", entity.ErrNotFound
	}
	return id.String(), nil
}

// parsePage reads and range-checks ?limit= and ?offset=, so the use case receives valid ints.
func parsePage(c *gin.Context) (limit, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, 0, errInvalidLimit
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, errInvalidOffset
	}
	return limit, offset, nil
}
