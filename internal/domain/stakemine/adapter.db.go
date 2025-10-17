// // internal/domain/stakemine/adapter.db.go
// package stakemine

// import (
// 	"github.com/esc-chula/intania-888-backend/internal/model"
// 	"gorm.io/gorm"
// )

// type stakeMineRepositoryImpl struct {
// 	db *gorm.DB
// }

// func NewStakeMineRepository(db *gorm.DB) StakeMineRepository {
// 	return &stakeMineRepositoryImpl{db: db}
// }

// func (r *stakeMineRepositoryImpl) Create(game *model.MineGame) error {
// 	return r.db.Create(game).Error
// }

// func (r *stakeMineRepositoryImpl) Update(game *model.MineGame) error {
// 	return r.db.Save(game).Error
// }

// func (r *stakeMineRepositoryImpl) FindById(gameId string) (*model.MineGame, error) {
// 	var game model.MineGame
// 	err := r.db.Where("id = ?", gameId).First(&game).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &game, nil
// }

// func (r *stakeMineRepositoryImpl) FindActiveByUserId(userId string) (*model.MineGame, error) {
// 	var game model.MineGame
// 	err := r.db.Where("user_id = ? AND status = ?", userId, "active").First(&game).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &game, nil
// }

// func (r *stakeMineRepositoryImpl) FindByUserId(userId string, limit int, offset int) ([]model.MineGame, error) {
// 	var games []model.MineGame
// 	err := r.db.Where("user_id = ?", userId).
// 		Order("created_at DESC").
// 		Limit(limit).
// 		Offset(offset).
// 		Find(&games).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return games, nil
// }

// func (r *stakeMineRepositoryImpl) CreateHistory(history *model.MineGameHistory) error {
// 	return r.db.Create(history).Error
// }

// func (r *stakeMineRepositoryImpl) GetStatsByUserId(userId string) (*model.MineGameStatsDto, error) {
// 	var stats model.MineGameStatsDto

// 	// Count games by status
// 	var gamesWon, gamesLost, gamesCashedOut int64
// 	r.db.Model(&model.MineGame{}).Where("user_id = ? AND status = ?", userId, "won").Count(&gamesWon)
// 	r.db.Model(&model.MineGame{}).Where("user_id = ? AND status = ?", userId, "lost").Count(&gamesLost)
// 	r.db.Model(&model.MineGame{}).Where("user_id = ? AND status = ?", userId, "cashed_out").Count(&gamesCashedOut)

// 	stats.GamesWon = int(gamesWon)
// 	stats.GamesLost = int(gamesLost)
// 	stats.GamesCashedOut = int(gamesCashedOut)
// 	stats.TotalGames = stats.GamesWon + stats.GamesLost + stats.GamesCashedOut

// 	// Calculate total wagered
// 	r.db.Model(&model.MineGame{}).
// 		Where("user_id = ?", userId).
// 		Select("COALESCE(SUM(bet_amount), 0)").
// 		Scan(&stats.TotalWagered)

// 	// Calculate total winnings (won + cashed out games only)
// 	r.db.Model(&model.MineGame{}).
// 		Where("user_id = ? AND status IN ?", userId, []string{"won", "cashed_out"}).
// 		Select("COALESCE(SUM(current_payout), 0)").
// 		Scan(&stats.TotalWinnings)

// 	stats.NetProfit = stats.TotalWinnings - stats.TotalWagered

// 	// Calculate win rate
// 	if stats.TotalGames > 0 {
// 		stats.WinRate = float64(stats.GamesWon+stats.GamesCashedOut) / float64(stats.TotalGames) * 100
// 	}

// 	return &stats, nil
// }

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
	sql := `
		INSERT INTO mine_games (
			id, user_id, bet_amount, risk_level, status,
			revealed_count, current_payout, multiplier, grid_data,
			created_at, updated_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW(), $10)
	`
	return r.db.Exec(sql,
		game.Id, game.UserId, game.BetAmount, game.RiskLevel, game.Status,
		game.RevealedCount, game.CurrentPayout, game.Multiplier, game.GridData, game.CompletedAt,
	).Error
}

func (r *stakeMineRepositoryImpl) Update(game *model.MineGame) error {
	sql := `
		UPDATE mine_games
		SET user_id = $2,
			bet_amount = $3,
			risk_level = $4,
			status = $5,
			revealed_count = $6,
			current_payout = $7,
			multiplier = $8,
			grid_data = $9,
			completed_at = $10,
			updated_at = NOW()
		WHERE id = $1
	`
	return r.db.Exec(sql,
		game.Id, game.UserId, game.BetAmount, game.RiskLevel, game.Status,
		game.RevealedCount, game.CurrentPayout, game.Multiplier, game.GridData, game.CompletedAt,
	).Error
}

func (r *stakeMineRepositoryImpl) FindById(gameId string) (*model.MineGame, error) {
	var game model.MineGame
	sql := `SELECT * FROM mine_games WHERE id = $1 LIMIT 1`
	if err := r.db.Raw(sql, gameId).Scan(&game).Error; err != nil {
		return nil, err
	}
	return &game, nil
}

func (r *stakeMineRepositoryImpl) FindActiveByUserId(userId string) (*model.MineGame, error) {
	var game model.MineGame
	sql := `
		SELECT * FROM mine_games 
		WHERE user_id = $1 AND status = 'active'
		LIMIT 1
	`
	result := r.db.Raw(sql, userId).Scan(&game)

	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, nil
	}

	return &game, nil
}

func (r *stakeMineRepositoryImpl) FindByUserId(userId string, limit int, offset int) ([]model.MineGame, error) {
	var games []model.MineGame
	sql := `
		SELECT * FROM mine_games 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	if err := r.db.Raw(sql, userId, limit, offset).Scan(&games).Error; err != nil {
		return nil, err
	}
	return games, nil
}

func (r *stakeMineRepositoryImpl) CreateHistory(history *model.MineGameHistory) error {
	sql := `
		INSERT INTO mine_game_histories (
			id, game_id, tile_index, tile_type, multiplier, payout_at_hit, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	return r.db.Exec(sql,
		history.Id, history.GameId, history.TileIndex,
		history.TileType, history.Multiplier, history.PayoutAtHit,
	).Error
}

func (r *stakeMineRepositoryImpl) GetStatsByUserId(userId string) (*model.MineGameStatsDto, error) {
	var stats model.MineGameStatsDto

	// Games by status
	r.db.Raw(`SELECT COUNT(*) FROM mine_games WHERE user_id = $1 AND status = 'won'`, userId).Scan(&stats.GamesWon)
	r.db.Raw(`SELECT COUNT(*) FROM mine_games WHERE user_id = $1 AND status = 'lost'`, userId).Scan(&stats.GamesLost)
	r.db.Raw(`SELECT COUNT(*) FROM mine_games WHERE user_id = $1 AND status = 'cashed_out'`, userId).Scan(&stats.GamesCashedOut)

	stats.TotalGames = stats.GamesWon + stats.GamesLost + stats.GamesCashedOut

	// Total wagered
	r.db.Raw(`SELECT COALESCE(SUM(bet_amount), 0) FROM mine_games WHERE user_id = $1`, userId).Scan(&stats.TotalWagered)

	// Total winnings (won + cashed_out)
	r.db.Raw(`
		SELECT COALESCE(SUM(current_payout), 0)
		FROM mine_games
		WHERE user_id = $1 AND status IN ('won', 'cashed_out')
	`, userId).Scan(&stats.TotalWinnings)

	stats.NetProfit = stats.TotalWinnings - stats.TotalWagered

	// Win rate
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.GamesWon+stats.GamesCashedOut) / float64(stats.TotalGames) * 100
	}

	return &stats, nil
}
