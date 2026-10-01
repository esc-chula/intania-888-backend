package stakemine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

// Service implements game use cases without transport or database dependencies.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService constructs Stake Mines use cases using the supplied repository.
func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		log:  log,
	}
}

// CreateGame funds one active game while holding the account lock.
func (s *Service) CreateGame(ctx context.Context, userID string, input CreateInput) (*GameResult, error) {
	// Validate the requested bet and risk level before opening a transaction.
	if !ValidateBetAmount(input.BetAmount) {
		return nil, ErrInvalidGameRequest
	}

	// Validate risk level.
	if !ValidateRiskLevel(input.RiskLevel) {
		return nil, ErrInvalidGameRequest
	}

	var game *Game
	// Lock the user while checking the active-game limit and deducting the bet.
	err := s.repo.WithinTransaction(ctx, func(tx Transaction) error {
		balance, err := tx.LockUserBalance(ctx, userID)
		if err != nil {
			return err
		}

		activeGameCount, err := tx.CountActiveGames(ctx, userID)
		if err != nil {
			return err
		}
		if activeGameCount > 0 {
			return ErrGameConflict
		}
		if balance < input.BetAmount.MinorUnits() {
			return ErrInsufficientBalance
		}

		grid, err := GenerateGrid(input.RiskLevel)
		if err != nil {
			return fmt.Errorf("generate game grid: %w", err)
		}
		game = &Game{
			ID:            uuid.NewString(),
			UserID:        userID,
			BetAmount:     input.BetAmount.MinorUnits(),
			RiskLevel:     input.RiskLevel,
			Status:        "active",
			RevealedCount: 0,
			CurrentPayout: input.BetAmount.MinorUnits(),
			Multiplier:    1000000,
			Grid:          grid,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := tx.CreateGame(ctx, game); err != nil {
			return err
		}
		if err := tx.AdjustBalance(ctx, userID, -input.BetAmount.MinorUnits()); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Map the persisted game to the response after the transaction commits.
	s.log.Info("Game created successfully",
		zap.String("game_id", game.ID),
		zap.String("user_id", userID),
		zap.Int64("bet_minor", input.BetAmount.MinorUnits()))

	return gameResult(game, true), nil
}

// RevealTile commits a reveal, terminal payout, and history under the game row lock.
func (s *Service) RevealTile(ctx context.Context, userID, gameID string, input RevealInput) (*GameResult, string, error) {
	if !ValidateTileIndex(input.Index) {
		return nil, "", ErrInvalidGameRequest
	}
	var game *Game
	var message string

	// Get and lock the game inside the transaction so only one transition can win.
	err := s.repo.WithinTransaction(ctx, func(tx Transaction) error {
		var err error
		game, err = tx.LockGame(ctx, gameID)
		if err != nil {
			return err
		}

		// Verify ownership.
		if game.UserID != userID {
			return ErrGameForbidden
		}

		// Check game status while the row is locked.
		if game.Status != "active" {
			return ErrGameConflict
		}

		if game.GridError != nil {
			return fmt.Errorf("load game grid: %w", game.GridError)
		}

		grid := game.Grid
		if input.Index >= len(grid) {
			return ErrInvalidGameRequest
		}
		// Check if tile already revealed.
		if grid[input.Index].Revealed {
			return ErrGameConflict
		}

		// Reveal the tile.
		grid[input.Index].Revealed = true
		game.RevealedCount++

		// Calculate the resulting game state and whether winnings must be credited.
		needsBalanceUpdate := false
		tileType := grid[input.Index].Type
		if tileType == "bomb" {
			// Hit a bomb - game over.
			game.Status = "lost"
			game.CurrentPayout = 0
			now := time.Now()
			game.CompletedAt = &now

			// Reveal all tiles.
			for i := range grid {
				grid[i].Revealed = true
			}
			message = "💣 BOOM! You hit a bomb and lost!"
		} else {
			// Found a diamond.
			multiplier := CalculateMultiplier(game.RevealedCount, game.RiskLevel)
			game.Multiplier = multiplier.MicroUnits()

			// Calculate payout safely with overflow protection.
			payout, err := CalculatePayoutSafe(value.MustMoneyFromMinor(game.BetAmount), multiplier)
			if err != nil {
				return fmt.Errorf("calculate game payout: %w", err)
			}
			game.CurrentPayout = payout.MinorUnits()
			totalTiles := 16
			bombs := GetBombCount(game.RiskLevel)

			// Check if all safe tiles revealed (auto win).
			if game.RevealedCount == totalTiles-bombs {
				game.Status = "won"
				now := time.Now()
				game.CompletedAt = &now
				needsBalanceUpdate = true

				// Reveal all tiles.
				for i := range grid {
					grid[i].Revealed = true
				}
				message = fmt.Sprintf("🎉 Perfect! You found all diamonds! Won: %s coins", payout.String())
			} else {
				message = fmt.Sprintf("💎 Diamond found! Current payout: %s coins (%sx)", payout.String(), multiplier.String())
			}
		}
		game.Grid = grid
		game.UpdatedAt = time.Now()

		// Persist the game, payout, and history atomically.
		if err := tx.SaveGame(ctx, game); err != nil {
			return err
		}
		if needsBalanceUpdate {
			if err := tx.AdjustBalance(ctx, userID, game.CurrentPayout); err != nil {
				return err
			}
		}

		// Record history as part of the same transaction.
		if err := tx.CreateHistory(ctx, &History{
			ID:          uuid.NewString(),
			GameID:      game.ID,
			TileIndex:   input.Index,
			TileType:    tileType,
			Multiplier:  game.Multiplier,
			PayoutAtHit: game.CurrentPayout,
			CreatedAt:   time.Now(),
		}); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	if game.Status != "active" {
		s.log.Info(
			"Game completed",
			zap.String("game_id", gameID),
			zap.String("user_id", userID),
			zap.String("status", game.Status),
		)
	}

	return gameResult(game, game.Status == "active"), message, nil
}

// CashOut credits one owned active game after at least one tile has been revealed.
func (s *Service) CashOut(ctx context.Context, userID, gameID string) (*GameResult, error) {
	var game *Game
	// Get and lock the game inside the transaction so only one cash-out can win.
	err := s.repo.WithinTransaction(ctx, func(tx Transaction) error {
		var err error
		game, err = tx.LockGame(ctx, gameID)
		if err != nil {
			return err
		}

		// Verify ownership.
		if game.UserID != userID {
			return ErrGameForbidden
		}
		// Check game status while the row is locked.
		if game.Status != "active" {
			return ErrGameConflict
		}
		// Must reveal at least one tile.
		if game.RevealedCount == 0 {
			return ErrGameConflict
		}

		// Reveal all tiles.
		if game.GridError != nil {
			return fmt.Errorf("load game grid: %w", game.GridError)
		}
		for i := range game.Grid {
			game.Grid[i].Revealed = true
		}
		game.Status = "cashed_out"
		now := time.Now()
		game.CompletedAt = &now
		game.UpdatedAt = time.Now()

		// Persist the terminal state and credit the current payout atomically.
		if err := tx.SaveGame(ctx, game); err != nil {
			return err
		}
		if err := tx.AdjustBalance(ctx, userID, game.CurrentPayout); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.log.Info("Player cashed out",
		zap.String("game_id", gameID),
		zap.String("user_id", userID),
		zap.Int64("payout_minor", game.CurrentPayout))

	return gameResult(game, false), nil
}

// GetGame retrieves an owned game and hides unrevealed active tiles.
func (s *Service) GetGame(ctx context.Context, userID, gameID string) (*GameResult, error) {
	game, err := s.repo.FindByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game.UserID != userID {
		return nil, ErrGameForbidden
	}
	if game.GridError != nil {
		return nil, game.GridError
	}

	return gameResult(game, game.Status == "active"), nil
}

// GetActiveGame retrieves the account's active game with unrevealed tiles hidden.
func (s *Service) GetActiveGame(ctx context.Context, userID string) (*GameResult, error) {
	game, err := s.repo.FindActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if game.GridError != nil {
		return nil, game.GridError
	}

	return gameResult(game, true), nil
}

// GetGameHistory retrieves the existing game history page in newest-first order.
func (s *Service) GetGameHistory(ctx context.Context, userID string, limit, offset int) ([]HistoryResult, error) {
	games, err := s.repo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	history := make([]HistoryResult, len(games))
	for i, game := range games {
		history[i] = HistoryResult{
			GameID:        game.ID,
			BetAmount:     value.MustMoneyFromMinor(game.BetAmount),
			RiskLevel:     game.RiskLevel,
			Status:        game.Status,
			FinalPayout:   value.MustMoneyFromMinor(game.CurrentPayout),
			Multiplier:    value.MustRateFromMicro(game.Multiplier),
			RevealedCount: game.RevealedCount,
			CreatedAt:     game.CreatedAt,
			CompletedAt:   game.CompletedAt,
		}
	}

	return history, nil
}

// GetStats retrieves realized totals and active exposure for the account.
func (s *Service) GetStats(ctx context.Context, userID string) (*StatsResult, error) {
	stats, err := s.repo.GetStatsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Calculate win rate from realized games only.
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.GamesWon+stats.GamesCashedOut) / float64(stats.TotalGames) * 100
	}

	return stats, nil
}
