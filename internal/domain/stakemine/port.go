package stakemine

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
)

type StakeMineService interface {
	CreateGame(userId string, req *model.CreateMineGameRequest) (*model.MineGameDto, error)
	RevealTile(userId string, gameId string, req *model.RevealMineTileRequest) (*model.MineGameDto, string, error)
	CashOut(userId string, gameId string) (*model.MineGameDto, error)
	GetGame(userId string, gameId string) (*model.MineGameDto, error)
	GetActiveGame(userId string) (*model.MineGameDto, error)
	GetGameHistory(userId string, limit int, offset int) ([]model.MineGameHistoryDto, error)
	GetStats(userId string) (*model.MineGameStatsDto, error)
}

type StakeMineRepository interface {
	Create(game *model.MineGame) error
	Update(game *model.MineGame) error
	FindById(gameId string) (*model.MineGame, error)
	FindActiveByUserId(userId string) (*model.MineGame, error)
	FindByUserId(userId string, limit int, offset int) ([]model.MineGame, error)
	CreateHistory(history *model.MineGameHistory) error
	GetStatsByUserId(userId string) (*model.MineGameStatsDto, error)
}
