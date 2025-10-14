// internal/domain/stakemine/utils.go
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

// Pre-calculated multiplier lookup tables based on your probability data
var multiplierTable = map[string]map[int]float64{
	"low": { // Easy (σ=0.9, 2 gn) - 2 bombs, 14 diamonds
		0:  1.0,
		1:  1.03,
		2:  1.07,
		3:  1.12,
		4:  1.19,
		5:  1.29,
		6:  1.42,
		7:  1.59,
		8:  1.84,
		9:  2.21,
		10: 2.79,
		11: 3.77,
		12: 5.65,
		13: 10.17,
		14: 27.45,
	},
	"medium": { // Medium (σ=0.9, 4 gn) - 4 bombs, 12 diamonds
		0:  1.0,
		1:  1.20,
		2:  1.47,
		3:  1.86,
		4:  2.41,
		5:  3.26,
		6:  4.61,
		7:  6.91,
		8:  11.19,
		9:  20.15,
		10: 42.31,
		11: 114.24,
		12: 514.44,
	},
	"high": { // Hard (σ=0.9, 6 gn) - 6 bombs, 10 diamonds
		0:  1.0,
		1:  1.44,
		2:  2.16,
		3:  3.40,
		4:  5.69,
		5:  10.24,
		6:  20.27,
		7:  45.60,
		8:  123.10,
		9:  443.27,
		10: 2789.43,
	},
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

// CalculateMultiplier returns the pre-calculated multiplier from lookup table
func CalculateMultiplier(diamondsFound int, risk string) float64 {
	// Get multiplier from lookup table
	if riskTable, exists := multiplierTable[risk]; exists {
		if multiplier, exists := riskTable[diamondsFound]; exists {
			return multiplier
		}
	}

	// Fallback to 1.0 if not found (should never happen)
	return 1.0
}

// GetMaxDiamonds returns maximum diamonds for a risk level
func GetMaxDiamonds(risk string) int {
	return 16 - GetBombCount(risk)
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
