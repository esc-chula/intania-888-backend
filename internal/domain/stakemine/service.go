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
	if req.BetAmount < 1 {
		return nil, errors.New("bet amount must be at least 1 coin")
	}
	if req.BetAmount > 1000000 {
		return nil, errors.New("bet amount cannot exceed 1,000,000 coins")
	}

	// Validate risk level
	if !ValidateRiskLevel(req.RiskLevel) {
		s.log.Named("CreateGame").Error("Invalid risk level", zap.String("risk", req.RiskLevel))
		return nil, errors.New("invalid risk level. must be 'low', 'medium', or 'high'")
	}

	var game *model.MineGame

	err := s.userDB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userId).
			First(&user).Error; err != nil {
			s.log.Named("CreateGame").Error("User not found", zap.Error(err))
			return errors.New("user not found")
		}

		var activeGameCount int64
		if err := tx.Model(&model.MineGame{}).
			Where("user_id = ? AND status = ?", userId, "active").
			Count(&activeGameCount).Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to check active games", zap.Error(err))
			return errors.New("failed to check active games")
		}
		if activeGameCount > 0 {
			s.log.Named("CreateGame").Warn("User already has active game", zap.String("userId", userId))
			return errors.New("you already have an active game. please finish or cash out first")
		}

		if user.RemainingCoin < req.BetAmount {
			s.log.Named("CreateGame").Warn("Insufficient balance",
				zap.String("userId", userId),
				zap.Float64("balance", user.RemainingCoin),
				zap.Float64("bet", req.BetAmount))
			return errors.New("insufficient balance")
		}

		grid, err := GenerateGrid(req.RiskLevel)
		if err != nil {
			s.log.Named("CreateGame").Error("Failed to generate grid", zap.Error(err))
			return errors.New("failed to generate game grid")
		}

		gridJSON, err := GridToJSON(grid)
		if err != nil {
			s.log.Named("CreateGame").Error("Failed to save grid", zap.Error(err))
			return errors.New("failed to save game data")
		}

		game = &model.MineGame{
			Id:            uuid.New().String(),
			UserId:        userId,
			BetAmount:     req.BetAmount,
			RiskLevel:     req.RiskLevel,
			Status:        "active",
			RevealedCount: 0,
			CurrentPayout: req.BetAmount,
			Multiplier:    1.0,
			GridData:      gridJSON,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := tx.Create(game).Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to create game", zap.Error(err))
			return errors.New("failed to create game")
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", gorm.Expr("remaining_coin - ?", req.BetAmount)).
			Error; err != nil {
			s.log.Named("CreateGame").Error("Failed to deduct balance", zap.Error(err))
			return errors.New("failed to deduct balance")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.log.Named("CreateGame").Info("Game created successfully",
		zap.String("gameId", game.Id),
		zap.String("userId", userId),
		zap.Float64("bet", req.BetAmount))
	return s.gameToDto(game, true)
}

func (s *stakeMineServiceImpl) RevealTile(userId string, gameId string, req *model.RevealMineTileRequest) (*model.MineGameDto, string, error) {
	// Get game
	game, err := s.repo.FindById(gameId)
	if err != nil {
		s.log.Named("RevealTile").Error("Game not found", zap.Error(err))
		return nil, "", errors.New("game not found")
	}

	// Verify ownership
	if game.UserId != userId {
		s.log.Named("RevealTile").Warn("Unauthorized access attempt", zap.String("userId", userId), zap.String("gameId", gameId))
		return nil, "", errors.New("unauthorized: this is not your game")
	}

	// Check game status
	if game.Status != "active" {
		return nil, "", errors.New("game is not active")
	}

	// Validate tile index
	if !ValidateTileIndex(req.Index) {
		return nil, "", errors.New("invalid tile index")
	}

	// Parse grid
	grid, err := JSONToGrid(game.GridData)
	if err != nil {
		s.log.Named("RevealTile").Error("Failed to parse grid", zap.Error(err))
		return nil, "", errors.New("failed to load game data")
	}

	// Check if tile already revealed
	if grid[req.Index].Revealed {
		return nil, "", errors.New("tile already revealed")
	}

	// Reveal the tile
	grid[req.Index].Revealed = true
	game.RevealedCount++

	var message string
	var needsBalanceUpdate bool

	if grid[req.Index].Type == "bomb" {
		// Hit a bomb - game over
		game.Status = "lost"
		game.CurrentPayout = 0
		now := time.Now()
		game.CompletedAt = &now

		// Reveal all tiles
		for i := range grid {
			grid[i].Revealed = true
		}

		message = "💣 BOOM! You hit a bomb and lost!"
		s.log.Named("RevealTile").Info("Player hit bomb", zap.String("gameId", gameId), zap.String("userId", userId))
	} else {
		// Found a diamond
		game.Multiplier = CalculateMultiplier(game.RevealedCount, game.RiskLevel)

		// Calculate payout safely with overflow protection
		payout, err := CalculatePayoutSafe(game.BetAmount, game.Multiplier)
		if err != nil {
			s.log.Named("RevealTile").Error("Payout calculation error", zap.Error(err))
			return nil, "", errors.New("payout calculation failed")
		}
		game.CurrentPayout = payout

		// Check if all safe tiles revealed (auto win)
		totalTiles := 16
		bombs := GetBombCount(game.RiskLevel)
		if game.RevealedCount == totalTiles-bombs {
			game.Status = "won"
			now := time.Now()
			game.CompletedAt = &now
			needsBalanceUpdate = true

			// Reveal all tiles
			for i := range grid {
				grid[i].Revealed = true
			}

			message = fmt.Sprintf("🎉 Perfect! You found all diamonds! Won: %.2f coins", game.CurrentPayout)
			s.log.Named("RevealTile").Info("Player won game", zap.String("gameId", gameId), zap.String("userId", userId), zap.Float64("payout", game.CurrentPayout))
		} else {
			message = fmt.Sprintf("💎 Diamond found! Current payout: %.2f coins (%.2fx)", game.CurrentPayout, game.Multiplier)
		}
	}

	gridJSON, _ := GridToJSON(grid)
	game.GridData = gridJSON
	game.UpdatedAt = time.Now()

	if needsBalanceUpdate {
		err := s.userDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(game).Error; err != nil {
				s.log.Named("RevealTile").Error("Failed to update game", zap.Error(err))
				return errors.New("failed to update game")
			}

			if err := tx.Model(&model.User{}).
				Where("id = ?", userId).
				Update("remaining_coin", gorm.Expr("remaining_coin + ?", game.CurrentPayout)).
				Error; err != nil {
				s.log.Named("RevealTile").Error("Failed to credit winnings", zap.Error(err))
				return errors.New("failed to credit winnings")
			}

			history := &model.MineGameHistory{
				Id:          uuid.New().String(),
				GameId:      game.Id,
				TileIndex:   req.Index,
				TileType:    "diamond",
				Multiplier:  game.Multiplier,
				PayoutAtHit: game.CurrentPayout,
				CreatedAt:   time.Now(),
			}
			if err := tx.Create(history).Error; err != nil {
				s.log.Named("RevealTile").Error("Failed to create history", zap.Error(err))
			}

			return nil
		})

		if err != nil {
			return nil, "", err
		}
	} else {
		if err := s.repo.Update(game); err != nil {
			s.log.Named("RevealTile").Error("Failed to update game", zap.Error(err))
			return nil, "", errors.New("failed to update game")
		}

		tileType := "diamond"
		if grid[req.Index].Type == "bomb" {
			tileType = "bomb"
		}
		s.repo.CreateHistory(&model.MineGameHistory{
			Id:          uuid.New().String(),
			GameId:      game.Id,
			TileIndex:   req.Index,
			TileType:    tileType,
			Multiplier:  game.Multiplier,
			PayoutAtHit: game.CurrentPayout,
			CreatedAt:   time.Now(),
		})
	}

	gameDto, _ := s.gameToDto(game, game.Status == "active")
	return gameDto, message, nil
}

func (s *stakeMineServiceImpl) CashOut(userId string, gameId string) (*model.MineGameDto, error) {
	// Get game
	game, err := s.repo.FindById(gameId)
	if err != nil {
		s.log.Named("CashOut").Error("Game not found", zap.Error(err))
		return nil, errors.New("game not found")
	}

	// Verify ownership
	if game.UserId != userId {
		s.log.Named("CashOut").Warn("Unauthorized cash out attempt", zap.String("userId", userId), zap.String("gameId", gameId))
		return nil, errors.New("unauthorized: this is not your game")
	}

	// Check game status
	if game.Status != "active" {
		return nil, errors.New("cannot cash out - game is not active")
	}

	// Must reveal at least one tile
	if game.RevealedCount == 0 {
		return nil, errors.New("cannot cash out without revealing any tiles")
	}

	// Update game status
	game.Status = "cashed_out"
	now := time.Now()
	game.CompletedAt = &now

	// Reveal all tiles
	grid, _ := JSONToGrid(game.GridData)
	for i := range grid {
		grid[i].Revealed = true
	}
	gridJSON, _ := GridToJSON(grid)
	game.GridData = gridJSON
	game.UpdatedAt = time.Now()

	err = s.userDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(game).Error; err != nil {
			s.log.Named("CashOut").Error("Failed to update game", zap.Error(err))
			return errors.New("failed to update game")
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userId).
			Update("remaining_coin", gorm.Expr("remaining_coin + ?", game.CurrentPayout)).
			Error; err != nil {
			s.log.Named("CashOut").Error("Failed to credit winnings", zap.Error(err))
			return errors.New("failed to credit winnings")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.log.Named("CashOut").Info("Player cashed out", zap.String("gameId", gameId), zap.String("userId", userId), zap.Float64("payout", game.CurrentPayout))
	return s.gameToDto(game, false)
}

func (s *stakeMineServiceImpl) GetGame(userId string, gameId string) (*model.MineGameDto, error) {
	game, err := s.repo.FindById(gameId)
	if err != nil {
		return nil, errors.New("game not found")
	}

	if game.UserId != userId {
		return nil, errors.New("unauthorized")
	}

	return s.gameToDto(game, game.Status == "active")
}

func (s *stakeMineServiceImpl) GetActiveGame(userId string) (*model.MineGameDto, error) {
	game, err := s.repo.FindActiveByUserId(userId)
	if err != nil {
		return nil, errors.New("no active game found")
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
			BetAmount:     game.BetAmount,
			RiskLevel:     game.RiskLevel,
			Status:        game.Status,
			FinalPayout:   game.CurrentPayout,
			Multiplier:    game.Multiplier,
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
		BetAmount:     game.BetAmount,
		RiskLevel:     game.RiskLevel,
		Grid:          tiles,
		RevealedCount: game.RevealedCount,
		CurrentPayout: game.CurrentPayout,
		Multiplier:    game.Multiplier,
		Status:        game.Status,
		CreatedAt:     game.CreatedAt,
		CompletedAt:   game.CompletedAt,
	}, nil
}
