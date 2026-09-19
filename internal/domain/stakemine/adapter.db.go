// internal/domain/stakemine/adapter.db.go
package stakemine

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"gorm.io/gorm"
)

type stakeMineRepositoryImpl struct {
	db *gorm.DB
}

func NewStakeMineRepository(db *gorm.DB) StakeMineRepository {
	return &stakeMineRepositoryImpl{db: db}
}

func (r *stakeMineRepositoryImpl) Create(game *model.MineGame) error {
	return r.db.Create(game).Error
}

func (r *stakeMineRepositoryImpl) Update(game *model.MineGame) error {
	return r.db.Save(game).Error
}

func (r *stakeMineRepositoryImpl) FindById(gameId string) (*model.MineGame, error) {
	var game model.MineGame

	err := r.db.Where("id = ?", gameId).First(&game).Error
	if err != nil {
		return nil, err
	}

	return &game, nil
}

func (r *stakeMineRepositoryImpl) FindActiveByUserId(userId string) (*model.MineGame, error) {
	var game model.MineGame

	err := r.db.Where("user_id = ? AND status = ?", userId, "active").First(&game).Error
	if err != nil {
		return nil, err
	}

	return &game, nil
}

func (r *stakeMineRepositoryImpl) FindByUserId(userId string, limit int, offset int) ([]model.MineGame, error) {
	var games []model.MineGame

	err := r.db.Where("user_id = ?", userId).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&games).Error
	if err != nil {
		return nil, err
	}

	return games, nil
}

func (r *stakeMineRepositoryImpl) CreateHistory(history *model.MineGameHistory) error {
	return r.db.Create(history).Error
}

func (r *stakeMineRepositoryImpl) GetStatsByUserId(userId string) (*model.MineGameStatsDto, error) {
	var stats model.MineGameStatsDto

	// Count games by status
	var gamesWon, gamesLost, gamesCashedOut int64

	if err := r.db.
		Model(&model.MineGame{}).
		Where("user_id = ? AND status = ?", userId, "won").
		Count(&gamesWon).
		Error; err != nil {
		return nil, err
	}

	if err := r.db.
		Model(&model.MineGame{}).
		Where("user_id = ? AND status = ?", userId, "lost").
		Count(&gamesLost).
		Error; err != nil {
		return nil, err
	}

	if err := r.db.
		Model(&model.MineGame{}).
		Where("user_id = ? AND status = ?", userId, "cashed_out").
		Count(&gamesCashedOut).
		Error; err != nil {
		return nil, err
	}

	stats.GamesWon = int(gamesWon)
	stats.GamesLost = int(gamesLost)
	stats.GamesCashedOut = int(gamesCashedOut)
	stats.TotalGames = stats.GamesWon + stats.GamesLost + stats.GamesCashedOut

	var totalWagered, totalWinnings int64

	// Realized totals exclude the active game.
	if err := r.db.Model(&model.MineGame{}).
		Where("user_id = ? AND status <> 'active'", userId).
		Select("COALESCE(SUM(bet_amount), 0)").
		Scan(&totalWagered).Error; err != nil {
		return nil, err
	}

	// Calculate total winnings (won + cashed out games only)
	if err := r.db.Model(&model.MineGame{}).
		Where("user_id = ? AND status IN ?", userId, []string{"won", "cashed_out"}).
		Select("COALESCE(SUM(current_payout), 0)").
		Scan(&totalWinnings).Error; err != nil {
		return nil, err
	}

	stats.TotalWagered = model.MustMoneyFromMinor(totalWagered)
	stats.TotalWinnings = model.MustMoneyFromMinor(totalWinnings)
	stats.NetProfit = model.NewSignedMoneyFromMinor(totalWinnings - totalWagered)

	// Include the currently active game's exposure separately.
	var active model.MineGame

	if err := r.db.
		Where("user_id = ? AND status = 'active'", userId).
		First(&active).
		Error; err == nil {
		w := model.MustMoneyFromMinor(active.BetAmount)
		p := model.MustMoneyFromMinor(active.CurrentPayout)

		stats.ActiveWagered = &w
		stats.ActiveCurrentPayout = &p
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Calculate win rate
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.GamesWon+stats.GamesCashedOut) / float64(stats.TotalGames) * 100
	}

	return &stats, nil
}
