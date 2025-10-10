package stakemine

import (
	"errors"
	"fmt"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	// Validate risk level
	if !ValidateRiskLevel(req.RiskLevel) {
		s.log.Named("CreateGame").Error("Invalid risk level", zap.String("risk", req.RiskLevel))
		return nil, errors.New("invalid risk level. must be 'low', 'medium', or 'high'")
	}

	// Check if user has an active game
	activeGame, _ := s.repo.FindActiveByUserId(userId)
	if activeGame != nil {
		s.log.Named("CreateGame").Warn("User already has active game", zap.String("userId", userId))
		return nil, errors.New("you already have an active game. please finish or cash out first")
	}

	// Check user balance
	var user model.User
	if err := s.userDB.Where("id = ?", userId).First(&user).Error; err != nil {
		s.log.Named("CreateGame").Error("User not found", zap.Error(err))
		return nil, errors.New("user not found")
	}

	if user.RemainingCoin < req.BetAmount {
		s.log.Named("CreateGame").Warn("Insufficient balance", zap.String("userId", userId), zap.Float64("balance", user.RemainingCoin), zap.Float64("bet", req.BetAmount))
		return nil, errors.New("insufficient balance")
	}

	// Deduct bet amount from user balance
	user.RemainingCoin -= req.BetAmount
	if err := s.userDB.Save(&user).Error; err != nil {
		s.log.Named("CreateGame").Error("Failed to deduct balance", zap.Error(err))
		return nil, errors.New("failed to deduct balance")
	}

	// Generate grid
	grid, err := GenerateGrid(req.RiskLevel)
	if err != nil {
		// Refund on error
		user.RemainingCoin += req.BetAmount
		s.userDB.Save(&user)
		s.log.Named("CreateGame").Error("Failed to generate grid", zap.Error(err))
		return nil, errors.New("failed to generate game grid")
	}

	gridJSON, err := GridToJSON(grid)
	if err != nil {
		// Refund on error
		user.RemainingCoin += req.BetAmount
		s.userDB.Save(&user)
		s.log.Named("CreateGame").Error("Failed to save grid", zap.Error(err))
		return nil, errors.New("failed to save game data")
	}

	// Create game
	game := &model.MineGame{
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

	if err := s.repo.Create(game); err != nil {
		// Refund on error
		user.RemainingCoin += req.BetAmount
		s.userDB.Save(&user)
		s.log.Named("CreateGame").Error("Failed to create game", zap.Error(err))
		return nil, errors.New("failed to create game")
	}

	s.log.Named("CreateGame").Info("Game created successfully", zap.String("gameId", game.Id), zap.String("userId", userId), zap.Float64("bet", req.BetAmount))
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

		// Record history
		s.repo.CreateHistory(&model.MineGameHistory{
			Id:          uuid.New().String(),
			GameId:      game.Id,
			TileIndex:   req.Index,
			TileType:    "bomb",
			Multiplier:  game.Multiplier,
			PayoutAtHit: 0,
			CreatedAt:   time.Now(),
		})
	} else {
		// Found a diamond
		game.Multiplier = CalculateMultiplier(game.RevealedCount, game.RiskLevel)
		game.CurrentPayout = game.BetAmount * game.Multiplier

		// Check if all safe tiles revealed (auto win)
		totalTiles := 16
		bombs := GetBombCount(game.RiskLevel)
		if game.RevealedCount == totalTiles-bombs {
			game.Status = "won"
			now := time.Now()
			game.CompletedAt = &now

			// Reveal all tiles
			for i := range grid {
				grid[i].Revealed = true
			}

			// Credit winnings to user
			var user model.User
			if err := s.userDB.Where("id = ?", userId).First(&user).Error; err == nil {
				user.RemainingCoin += game.CurrentPayout
				s.userDB.Save(&user)
			}

			message = fmt.Sprintf("🎉 Perfect! You found all diamonds! Won: %.2f coins", game.CurrentPayout)
			s.log.Named("RevealTile").Info("Player won game", zap.String("gameId", gameId), zap.String("userId", userId), zap.Float64("payout", game.CurrentPayout))
		} else {
			message = fmt.Sprintf("💎 Diamond found! Current payout: %.2f coins (%.2fx)", game.CurrentPayout, game.Multiplier)
		}

		// Record history
		s.repo.CreateHistory(&model.MineGameHistory{
			Id:          uuid.New().String(),
			GameId:      game.Id,
			TileIndex:   req.Index,
			TileType:    "diamond",
			Multiplier:  game.Multiplier,
			PayoutAtHit: game.CurrentPayout,
			CreatedAt:   time.Now(),
		})
	}

	// Save updated grid
	gridJSON, _ := GridToJSON(grid)
	game.GridData = gridJSON
	game.UpdatedAt = time.Now()

	if err := s.repo.Update(game); err != nil {
		s.log.Named("RevealTile").Error("Failed to update game", zap.Error(err))
		return nil, "", errors.New("failed to update game")
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

	// Credit winnings to user
	var user model.User
	if err := s.userDB.Where("id = ?", userId).First(&user).Error; err != nil {
		s.log.Named("CashOut").Error("User not found during cash out", zap.Error(err))
		return nil, errors.New("user not found")
	}

	user.RemainingCoin += game.CurrentPayout
	if err := s.userDB.Save(&user).Error; err != nil {
		s.log.Named("CashOut").Error("Failed to credit winnings", zap.Error(err))
		return nil, errors.New("failed to credit winnings")
	}

	if err := s.repo.Update(game); err != nil {
		s.log.Named("CashOut").Error("Failed to update game", zap.Error(err))
		return nil, errors.New("failed to update game")
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
