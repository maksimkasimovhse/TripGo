package finish

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	repository "github.com/maksimkasimovhse/TripGo/internal/repository/trip"
)

type Usecase struct {
	txm          TxManager
	tripRepo     *repository.TripRepository
	historyRepo  *repository.StatusHistoryRepository
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

func (u *Usecase) Finish() {

}
