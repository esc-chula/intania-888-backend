package stakemine

import (
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type stakeMineServiceImpl struct {
	repo   StakeMineRepository
	userDB *gorm.DB
	log    *zap.Logger
}

func NewStakeMineService(repo StakeMineRepository, db *gorm.DB, log *zap.Logger) StakeMineService {
	return &stakeMineServiceImpl{
		repo:   repo,
		userDB: db,
		log:    log,
	}
}

func (s *stakeMineServiceImpl) CreateGame(userId string, req *model.CreateMineGameRequest) (*model.MineGameDto, error) {
	// Validate the requested bet and risk level before opening a transaction.
	if req == nil || req.BetAmount.MinorUnits() < 1_00 {
		return nil, ErrInvalidGameRequest
	}

	if req.BetAmount.MinorUnits() > 1_000_000_00 {
		return nil, ErrInvalidGameRequest
	}

	// Validate risk level
	if !ValidateRiskLevel(req.RiskLevel) {
		s.log.Named("CreateGame").Error("Invalid risk level", zap.String("risk", req.RiskLevel))
		return nil, ErrInvalidGameRequest
	}

	var game *model.MineGame

	// Lock the user while checking the active-game limit and deducting the bet.
	err := s.userDB.Transaction(func(tx *gorm.DB) error {
		var user model.User

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			lookupErr := mapUserLookupError(err)
			if errors.Is(lookupErr, ErrUserNotFound) {
				s.log.Named("CreateGame").Warn("User not found", zap.String("user_id", userId))
			} else {
				s.log.Named("CreateGame").Error("Failed to load user", zap.Error(err))
			}
			return lookupErr
		}

		var activeGameCount int64

		if err := tx.Model(&model.MineGame{}).
			Where("user_id = ? AND status = ?", userId, "active").
			Count(&activeGameCount).Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to check active games", zap.Error(err))
			return fmt.Errorf("check active games: %w", err)
		}

		if activeGameCount > 0 {
			s.log.Named("CreateGame").Warn("User already has active game", zap.String("userId", userId))
			return ErrGameConflict
		}

		if user.RemainingCoin < req.BetAmount.MinorUnits() {
			s.log.Named("CreateGame").Warn("Insufficient balance",
				zap.String("userId", userId),
				zap.Int64("balance_minor", user.RemainingCoin),
				zap.Int64("bet_minor", req.BetAmount.MinorUnits()))
			return ErrInsufficientBalance
		}

		grid, err := GenerateGrid(req.RiskLevel)
		if err != nil {
			s.log.Named("CreateGame").Error("Failed to generate grid", zap.Error(err))
			return fmt.Errorf("generate game grid: %w", err)
		}

		gridJSON, err := GridToJSON(grid)
		if err != nil {
			s.log.Named("CreateGame").Error("Failed to save grid", zap.Error(err))
			return fmt.Errorf("serialize game grid: %w", err)
		}

		game = &model.MineGame{
			Id:            uuid.New().String(),
			UserId:        userId,
			BetAmount:     req.BetAmount.MinorUnits(),
			RiskLevel:     req.RiskLevel,
			Status:        "active",
			RevealedCount: 0,
			CurrentPayout: req.BetAmount.MinorUnits(),
			Multiplier:    1000000,
			GridData:      gridJSON,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := tx.Create(game).Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to create game", zap.Error(err))
			return fmt.Errorf("create game: %w", err)
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", req.BetAmount.MinorUnits())).
			Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to deduct balance", zap.Error(err))
			return fmt.Errorf("deduct game bet: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Map the persisted game to the response after the transaction commits.
	s.log.Named("CreateGame").Info("Game created successfully",
		zap.String("gameId", game.Id),
		zap.String("userId", userId),
		zap.Int64("bet_minor", req.BetAmount.MinorUnits()))
	return s.gameToDto(game, true)
}

func (s *stakeMineServiceImpl) RevealTile(userId string, gameId string, req *model.RevealMineTileRequest) (*model.MineGameDto, string, error) {
	if req == nil || !ValidateTileIndex(req.Index) {
		return nil, "", ErrInvalidGameRequest
	}
	var game model.MineGame
	var message string

	// Get and lock the game inside the transaction so only one transition can win.
	err := s.userDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", gameId).
			First(&game).
			Error; err != nil {
			s.log.Named("RevealTile").Error("Game not found", zap.Error(err))
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrGameNotFound
			}
			return err
		}

		// Verify ownership.
		if game.UserId != userId {
			s.log.Named("RevealTile").Warn("Unauthorized access attempt", zap.String("userId", userId), zap.String("gameId", gameId))
			return ErrGameForbidden
		}

		// Check game status while the row is locked.
		if game.Status != "active" {
			return ErrGameConflict
		}

		// Validate tile index.
		if !ValidateTileIndex(req.Index) {
			return ErrInvalidGameRequest
		}

		// Parse grid.
		grid, err := JSONToGrid(game.GridData)
		if err != nil || req.Index >= len(grid) {
			s.log.Named("RevealTile").Error("Failed to parse grid", zap.Error(err))
			if err != nil {
				return fmt.Errorf("load game grid: %w", err)
			}

			return ErrInvalidGameRequest
		}

		// Check if tile already revealed.
		if grid[req.Index].Revealed {
			return ErrGameConflict
		}

		// Reveal the tile.
		grid[req.Index].Revealed = true
		game.RevealedCount++

		// Calculate the resulting game state and whether winnings must be credited.
		needsBalanceUpdate := false
		tileType := grid[req.Index].Type

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
			s.log.Named("RevealTile").Info("Player hit bomb", zap.String("gameId", gameId), zap.String("userId", userId))
		} else {
			// Found a diamond.
			multiplier := CalculateMultiplier(game.RevealedCount, game.RiskLevel)
			game.Multiplier = multiplier.MicroUnits()

			// Calculate payout safely with overflow protection.
			payout, err := CalculatePayoutSafe(model.MustMoneyFromMinor(game.BetAmount), multiplier)
			if err != nil {
				s.log.Named("RevealTile").Error("Payout calculation error", zap.Error(err))
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
				s.log.Named("RevealTile").Info("Player won game", zap.String("gameId", gameId), zap.String("userId", userId), zap.Int64("payout_minor", game.CurrentPayout))
			} else {
				message = fmt.Sprintf("💎 Diamond found! Current payout: %s coins (%sx)", payout.String(), multiplier.String())
			}
		}

		gridJSON, err := GridToJSON(grid)
		if err != nil {
			s.log.Named("RevealTile").Error("Failed to serialize grid", zap.Error(err))
			return fmt.Errorf("serialize game grid: %w", err)
		}

		game.GridData = gridJSON
		game.UpdatedAt = time.Now()

		// Persist the game, payout, and history atomically.
		if err := tx.Save(&game).Error; err != nil {
			s.log.Named("RevealTile").Error("Failed to update game", zap.Error(err))
			return fmt.Errorf("persist game update: %w", err)
		}

		if needsBalanceUpdate {
			if err := tx.Model(&model.User{}).
				Where("id = ?", userId).
				Update("remaining_coin", gorm.Expr("remaining_coin + ?", game.CurrentPayout)).
				Error; err != nil {
				s.log.Named("RevealTile").Error("Failed to credit winnings", zap.Error(err))
				return fmt.Errorf("credit winnings: %w", err)
			}
		}

		// Record history as part of the same transaction.
		if err := tx.Create(&model.MineGameHistory{
			Id:          uuid.New().String(),
			GameId:      game.Id,
			TileIndex:   req.Index,
			TileType:    tileType,
			Multiplier:  game.Multiplier,
			PayoutAtHit: game.CurrentPayout,
			CreatedAt:   time.Now(),
		}).Error; err != nil {
			s.log.Named("RevealTile").Error("Failed to create history", zap.Error(err))
			return fmt.Errorf("create game history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	gameDto, err := s.gameToDto(&game, game.Status == "active")
	if err != nil {
		return nil, "", fmt.Errorf("serialize game response: %w", err)
	}

	return gameDto, message, nil
}

func (s *stakeMineServiceImpl) CashOut(userId string, gameId string) (*model.MineGameDto, error) {
	var game model.MineGame
	// Get and lock the game inside the transaction so only one cash-out can win.
	err := s.userDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", gameId).
			First(&game).
			Error; err != nil {
			s.log.Named("CashOut").Error("Game not found", zap.Error(err))
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrGameNotFound
			}
			return err
		}

		// Verify ownership.
		if game.UserId != userId {
			s.log.Named("CashOut").Warn("Unauthorized cash out attempt", zap.String("userId", userId), zap.String("gameId", gameId))
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
		grid, err := JSONToGrid(game.GridData)
		if err != nil {
			s.log.Named("CashOut").Error("Failed to parse grid", zap.Error(err))
			return fmt.Errorf("load game grid: %w", err)
		}

		for i := range grid {
			grid[i].Revealed = true
		}

		gridJSON, err := GridToJSON(grid)
		if err != nil {
			s.log.Named("CashOut").Error("Failed to serialize grid", zap.Error(err))
			return fmt.Errorf("serialize game grid: %w", err)
		}

		game.Status = "cashed_out"
		now := time.Now()
		game.CompletedAt = &now
		game.GridData = gridJSON
		game.UpdatedAt = time.Now()

		// Persist the terminal state and credit the current payout atomically.
		if err := tx.Save(&game).Error; err != nil {
			s.log.Named("CashOut").Error("Failed to update game", zap.Error(err))
			return fmt.Errorf("persist game update: %w", err)
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", gorm.Expr("remaining_coin + ?", game.CurrentPayout)).
			Error; err != nil {
			s.log.Named("CashOut").Error("Failed to credit winnings", zap.Error(err))
			return fmt.Errorf("credit winnings: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.log.Named("CashOut").Info("Player cashed out", zap.String("gameId", gameId), zap.String("userId", userId), zap.Int64("payout_minor", game.CurrentPayout))
	return s.gameToDto(&game, false)
}

func (s *stakeMineServiceImpl) GetGame(userId string, gameId string) (*model.MineGameDto, error) {
	game, err := s.repo.FindById(gameId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGameNotFound
		}
		return nil, err
	}

	if game.UserId != userId {
		return nil, ErrGameForbidden
	}

	return s.gameToDto(game, game.Status == "active")
}

func (s *stakeMineServiceImpl) GetActiveGame(userId string) (*model.MineGameDto, error) {
	game, err := s.repo.FindActiveByUserId(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoActiveGame
		}
		return nil, err
	}

	return s.gameToDto(game, true)
}

func (s *stakeMineServiceImpl) GetGameHistory(userId string, limit int, offset int) ([]model.MineGameHistoryDto, error) {
	games, err := s.repo.FindByUserId(userId, limit, offset)
	if err != nil {
		s.log.Named("GetGameHistory").Error("Failed to get history", zap.Error(err))
		return nil, err
	}

	history := make([]model.MineGameHistoryDto, len(games))
	for i, game := range games {
		history[i] = model.MineGameHistoryDto{
			GameId:        game.Id,
			BetAmount:     model.MustMoneyFromMinor(game.BetAmount),
			RiskLevel:     game.RiskLevel,
			Status:        game.Status,
			FinalPayout:   model.MustMoneyFromMinor(game.CurrentPayout),
			Multiplier:    model.MustRateFromMicro(game.Multiplier),
			RevealedCount: game.RevealedCount,
			CreatedAt:     game.CreatedAt,
			CompletedAt:   game.CompletedAt,
		}
	}

	return history, nil
}

func (s *stakeMineServiceImpl) GetStats(userId string) (*model.MineGameStatsDto, error) {
	return s.repo.GetStatsByUserId(userId)
}

// Helper function to convert game entity to DTO
func (s *stakeMineServiceImpl) gameToDto(game *model.MineGame, hideUnrevealed bool) (*model.MineGameDto, error) {
	grid, err := JSONToGrid(game.GridData)
	if err != nil {
		return nil, err
	}

	safeGrid := GetSafeGrid(grid, hideUnrevealed)
	tiles := make([]model.MineTileDto, len(safeGrid))

	for i, tile := range safeGrid {
		tiles[i] = model.MineTileDto{
			Index:    tile.Index,
			Type:     tile.Type,
			Revealed: tile.Revealed,
		}
	}

	return &model.MineGameDto{
		Id:            game.Id,
		UserId:        game.UserId,
		BetAmount:     model.MustMoneyFromMinor(game.BetAmount),
		RiskLevel:     game.RiskLevel,
		Grid:          tiles,
		RevealedCount: game.RevealedCount,
		CurrentPayout: model.MustMoneyFromMinor(game.CurrentPayout),
		Multiplier:    model.MustRateFromMicro(game.Multiplier),
		Status:        game.Status,
		CreatedAt:     game.CreatedAt,
		CompletedAt:   game.CompletedAt,
	}, nil
}

func mapUserLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.Join(ErrUserNotFound, err)
	}

	return err
}
