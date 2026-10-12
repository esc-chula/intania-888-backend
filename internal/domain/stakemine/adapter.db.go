package stakemine

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

type gormRepository struct{ db *gorm.DB }

// NewGORMRepository constructs a Stake Mines repository backed by PostgreSQL/GORM.
func NewGORMRepository(db *gorm.DB) *gormRepository {
	return &gormRepository{db: db}
}

// WithinTransaction provides game operations on one database transaction.
func (r *gormRepository) WithinTransaction(ctx context.Context, fn func(Transaction) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepository{db: tx})
	})
}

// LockUserBalance locks the account row and retrieves its balance.
func (r *gormRepository) LockUserBalance(ctx context.Context, userID string) (int64, error) {
	var user persistence.User
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", userID).First(&user).Error; err != nil {
		return 0, fmt.Errorf("lock game account: %w", mapUserLookupError(err))
	}

	return user.RemainingCoin, nil
}

// CountActiveGames counts active games under the account lock.
func (r *gormRepository) CountActiveGames(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&persistence.MineGame{}).
		Where("user_id = ? AND status = ?", userID, "active").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count active games: %w", err)
	}

	return count, nil
}

// LockGame locks a game row and translates its stored grid.
func (r *gormRepository) LockGame(ctx context.Context, gameID string) (*Game, error) {
	var row persistence.MineGame
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", gameID).First(&row).Error; err != nil {
		return nil, fmt.Errorf("lock game: %w", mapGameLookupError(err, ErrGameNotFound))
	}

	return gameSnapshot(&row, true)
}

// CreateGame persists a game and its stored grid.
func (r *gormRepository) CreateGame(ctx context.Context, game *Game) error {
	row, err := gameRow(game)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create game: %w", err)
	}
	game.CreatedAt = row.CreatedAt
	game.UpdatedAt = row.UpdatedAt

	return nil
}

// SaveGame persists the locked game state.
func (r *gormRepository) SaveGame(ctx context.Context, game *Game) error {
	row, err := gameRow(game)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Save(row).Error; err != nil {
		return fmt.Errorf("persist game update: %w", err)
	}
	game.UpdatedAt = row.UpdatedAt

	return nil
}

// AdjustBalance adds signed minor units to the account atomically.
func (r *gormRepository) AdjustBalance(ctx context.Context, userID string, delta int64) error {
	if err := r.db.WithContext(ctx).Model(&persistence.User{}).
		Where("id = ?", userID).
		Update("remaining_coin", gorm.Expr("remaining_coin + ?", delta)).Error; err != nil {
		return fmt.Errorf("adjust game balance: %w", err)
	}

	return nil
}

// CreateHistory persists one reveal within the game transaction.
func (r *gormRepository) CreateHistory(ctx context.Context, history *History) error {
	row := &persistence.MineGameHistory{
		ID:          history.ID,
		GameID:      history.GameID,
		TileIndex:   history.TileIndex,
		TileType:    history.TileType,
		Multiplier:  history.Multiplier,
		PayoutAtHit: history.PayoutAtHit,
		CreatedAt:   history.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create game history: %w", err)
	}

	return nil
}

// FindByID retrieves a game and translates its stored grid.
func (r *gormRepository) FindByID(ctx context.Context, gameID string) (*Game, error) {
	var row persistence.MineGame
	if err := r.db.WithContext(ctx).Where("id = ?", gameID).First(&row).Error; err != nil {
		return nil, fmt.Errorf("find game: %w", mapGameLookupError(err, ErrGameNotFound))
	}

	return gameSnapshot(&row, true)
}

// FindActiveByUserID retrieves the account's active game.
func (r *gormRepository) FindActiveByUserID(ctx context.Context, userID string) (*Game, error) {
	var row persistence.MineGame
	if err := r.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, "active").First(&row).Error; err != nil {
		return nil, fmt.Errorf("find active game: %w", mapGameLookupError(err, ErrNoActiveGame))
	}

	return gameSnapshot(&row, true)
}

// FindByUserID retrieves game history in newest-first order.
func (r *gormRepository) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]Game, error) {
	var rows []persistence.MineGame
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list game history: %w", err)
	}

	games := make([]Game, len(rows))
	for i := range rows {
		// History reads do not inspect stored grids, preserving the existing behavior.
		game, err := gameSnapshot(&rows[i], false)
		if err != nil {
			return nil, err
		}
		games[i] = *game
	}

	return games, nil
}

// GetStatsByUserID retrieves realized totals and active exposure.
func (r *gormRepository) GetStatsByUserID(ctx context.Context, userID string) (*StatsResult, error) {
	var stats StatsResult

	// Count games by status
	var gamesWon, gamesLost, gamesCashedOut int64

	if err := r.db.WithContext(ctx).
		Model(&persistence.MineGame{}).
		Where("user_id = ? AND status = ?", userID, "won").
		Count(&gamesWon).
		Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).
		Model(&persistence.MineGame{}).
		Where("user_id = ? AND status = ?", userID, "lost").
		Count(&gamesLost).
		Error; err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).
		Model(&persistence.MineGame{}).
		Where("user_id = ? AND status = ?", userID, "cashed_out").
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
	if err := r.db.WithContext(ctx).Model(&persistence.MineGame{}).
		Where("user_id = ? AND status <> 'active'", userID).
		Select("COALESCE(SUM(bet_amount), 0)").
		Scan(&totalWagered).Error; err != nil {
		return nil, err
	}

	// Calculate total winnings (won + cashed out games only)
	if err := r.db.WithContext(ctx).Model(&persistence.MineGame{}).
		Where("user_id = ? AND status IN ?", userID, []string{"won", "cashed_out"}).
		Select("COALESCE(SUM(current_payout), 0)").
		Scan(&totalWinnings).Error; err != nil {
		return nil, err
	}

	stats.TotalWagered = value.MustMoneyFromMinor(totalWagered)
	stats.TotalWinnings = value.MustMoneyFromMinor(totalWinnings)
	stats.NetProfit = value.NewSignedMoneyFromMinor(totalWinnings - totalWagered)

	// Include the currently active game's exposure separately.
	var active persistence.MineGame

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = 'active'", userID).
		First(&active).
		Error; err == nil {
		w := value.MustMoneyFromMinor(active.BetAmount)
		p := value.MustMoneyFromMinor(active.CurrentPayout)

		stats.ActiveWagered = &w
		stats.ActiveCurrentPayout = &p
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return &stats, nil
}

func mapUserLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.Join(ErrUserNotFound, err)
	}

	return err
}

func mapGameLookupError(err, missing error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.Join(missing, err)
	}

	return err
}
