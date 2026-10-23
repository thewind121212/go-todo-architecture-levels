package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level05/internal/usecase"
)

// ListActivitiesUseCase is the input port this controller needs.
type ListActivitiesUseCase interface {
	Execute(ctx context.Context, in usecase.ListActivitiesInput) (usecase.ListActivitiesOutput, error)
}

type ActivityController struct {
	list ListActivitiesUseCase
	log  zerolog.Logger
}

func NewActivityController(list ListActivitiesUseCase, log zerolog.Logger) *ActivityController {
	return &ActivityController{list: list, log: log}
}

func (ac *ActivityController) List(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, ac.log, err)
		return
	}
	out, err := ac.list.Execute(c.Request.Context(), usecase.ListActivitiesInput{TodoID: id})
	if err != nil {
		writeError(c, ac.log, err)
		return
	}
	c.JSON(http.StatusOK, presentActivityList(out))
}
