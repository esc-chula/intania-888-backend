// internal/domain/stakemine/utils.go
package stakemine

import (
	"crypto/rand"
	"encoding/json"
	"errors"
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
		3:  1.19,
		4:  1.33,
		5:  1.52,
		6:  1.76,
		7:  2.10,
		8:  2.56,
		9:  3.24,
		10: 4.31,
		11: 6.14,
		12: 9.73,
		13: 18.48,
		14: 52.67,
	},
	"medium": { // Medium (σ=0.9, 4 gn) - 4 bombs, 12 diamonds
		0:  1.0,
		1:  1.14,
		2:  1.48,
		3:  1.96,
		4:  2.70,
		5:  3.84,
		6:  5.73,
		7:  9.08,
		8:  15.52,
		9:  29.50,
		10: 65.38,
		11: 186.36,
		12: 885.84,
	},
	"high": { // Hard (σ=0.9, 6 gn) - 6 bombs, 10 diamonds
		0:  1.0,
		1:  1.37,
		2:  2.17,
		3:  3.60,
		4:  6.35,
		5:  12.07,
		6:  25.23,
		7:  59.91,
		8:  170.74,
		9:  649.00,
		10: 4310.91,
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

	//Fisher-Yates
	indices := make([]int, 16)
	for i := range indices {
		indices[i] = i
	}

	for i := 15; i > 0; i-- {
		j, err := SecureRandom(i + 1)
		if err != nil {
			return nil, err
		}
		indices[i], indices[j] = indices[j], indices[i]
	}

	for i := 0; i < bombCount; i++ {
		grid[indices[i]].Type = "bomb"
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

// ValidateBetAmount
func ValidateBetAmount(amount float64) bool {
	return amount >= 1 && amount <= 1000000
}

// CalculatePayoutSafe overflow protection
func CalculatePayoutSafe(betAmount float64, multiplier float64) (float64, error) {
	const maxFloat64 = 1.7976931348623157e+308
	if betAmount > 0 && multiplier > maxFloat64/betAmount {
		return 0, errors.New("payout calculation would overflow")
	}

	payout := betAmount * multiplier

	payout = roundToTwoDecimals(payout)

	return payout, nil
}

func roundToTwoDecimals(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
