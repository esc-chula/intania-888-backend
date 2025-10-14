package stakemine

import (
	"crypto/rand"
	"encoding/json"
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
		return 2 // Easy mode: 2 bombs, 14 diamonds
	case "medium":
		return 4 // Medium mode: 4 bombs, 12 diamonds
	case "high":
		return 6 // Hard mode: 6 bombs, 10 diamonds
	default:
		return 2
	}
}

// CalculateMultiplier calculates the payout multiplier based on diamonds found and risk level
// This matches the probability table provided
func CalculateMultiplier(diamondsFound int, risk string) float64 {
	if diamondsFound == 0 {
		return 1.0
	}

	totalTiles := 16
	bombs := GetBombCount(risk)
	diamonds := totalTiles - bombs

	// House edge factor (RTP = 0.9, so multiply by 0.9)
	houseEdge := 0.9

	// Calculate cumulative multiplier
	multiplier := 1.0

	for i := 0; i < diamondsFound; i++ {
		remainingDiamonds := diamonds - i
		remainingTiles := totalTiles - i

		// Probability of hitting a diamond = remainingDiamonds / remainingTiles
		probability := float64(remainingDiamonds) / float64(remainingTiles)

		// Multiplier increases by 1/probability for each successful pick
		multiplier *= (1.0 / probability)
	}

	// Apply house edge
	multiplier *= houseEdge

	// Round to 2 decimal places
	return float64(int(multiplier*100+0.5)) / 100
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
