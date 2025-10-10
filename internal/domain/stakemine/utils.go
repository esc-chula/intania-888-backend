package stakemine

import (
	"crypto/rand"
	"encoding/json"
	"math"
	"math/big"
)

// Tile represents a single tile in the grid
type Tile struct {
	Index    int    `json:"index"`
	Type     string `json:"type"` // diamond, bomb
	Revealed bool   `json:"revealed"`
}

// GetBombCount returns number of bombs based on risk level
func GetBombCount(risk string) int {
	switch risk {
	case "low":
		return 3 // 18.75% chance (3/16)
	case "medium":
		return 5 // 31.25% chance (5/16)
	case "high":
		return 7 // 43.75% chance (7/16)
	default:
		return 3
	}
}

// CalculateMultiplier calculates the payout multiplier based on diamonds found and risk level
func CalculateMultiplier(diamondsFound int, risk string) float64 {
	totalTiles := 16
	bombs := GetBombCount(risk)
	safeTiles := totalTiles - bombs

	if diamondsFound == 0 {
		return 1.0
	}

	// Calculate multiplier based on probability
	multiplier := 1.0
	remainingSafe := safeTiles
	remainingTotal := totalTiles

	for i := 0; i < diamondsFound; i++ {
		// Probability of picking safe tile
		prob := float64(remainingSafe) / float64(remainingTotal)
		// House edge: 4% (96% RTP)
		houseEdge := 0.96
		multiplier *= (1.0 / prob) * houseEdge

		remainingSafe--
		remainingTotal--
	}

	// Round to 2 decimal places
	return math.Round(multiplier*100) / 100
}

// SecureRandom generates a cryptographically secure random number between 0 and max-1
func SecureRandom(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// GenerateGrid creates a new game grid with randomly placed bombs
func GenerateGrid(risk string) ([]Tile, error) {
	bombCount := GetBombCount(risk)
	grid := make([]Tile, 16)

	// Initialize all tiles as diamonds
	for i := 0; i < 16; i++ {
		grid[i] = Tile{
			Index:    i,
			Type:     "diamond",
			Revealed: false,
		}
	}

	// Randomly place bombs using Fisher-Yates shuffle approach
	bombsPlaced := 0
	attempts := 0
	maxAttempts := 100

	for bombsPlaced < bombCount && attempts < maxAttempts {
		idx, err := SecureRandom(16)
		if err != nil {
			return nil, err
		}

		if grid[idx].Type != "bomb" {
			grid[idx].Type = "bomb"
			bombsPlaced++
		}
		attempts++
	}

	return grid, nil
}

// GridToJSON converts grid to JSON string for database storage
func GridToJSON(grid []Tile) (string, error) {
	data, err := json.Marshal(grid)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSONToGrid converts JSON string back to grid
func JSONToGrid(jsonStr string) ([]Tile, error) {
	var grid []Tile
	err := json.Unmarshal([]byte(jsonStr), &grid)
	if err != nil {
		return nil, err
	}
	return grid, nil
}

// GetSafeGrid returns grid with hidden tiles for active games
func GetSafeGrid(grid []Tile, isActive bool) []Tile {
	safeGrid := make([]Tile, len(grid))
	for i, tile := range grid {
		safeTile := tile
		// Hide unrevealed tiles if game is still active
		if !tile.Revealed && isActive {
			safeTile.Type = "hidden"
		}
		safeGrid[i] = safeTile
	}
	return safeGrid
}

// ValidateRiskLevel checks if risk level is valid
func ValidateRiskLevel(risk string) bool {
	return risk == "low" || risk == "medium" || risk == "high"
}

// ValidateTileIndex checks if tile index is valid
func ValidateTileIndex(index int) bool {
	return index >= 0 && index < 16
}
