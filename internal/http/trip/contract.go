package trip

import (
	"context"

	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/get"
)

type Handler struct {
	createUsecase *create.Usecase
	getUsecase    *get.Usecase
	finishUsecase *finish.Usecase
}

func New(Create *create.Usecase, get *get.Usecase, finish *finish.Usecase) *Handler {
	return &Handler{createUsecase: Create, getUsecase: get, finishUsecase: finish}
}
