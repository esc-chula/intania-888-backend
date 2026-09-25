// internal/domain/stakemine/utils.go
package stakemine

import (
	"crypto/rand"
	"encoding/json"
	"math/big"

	"github.com/esc-chula/intania-888-backend/internal/model"
)

// Tile represents a single tile in the grid
type Tile struct {
	Index    int    `json:"index"`
	Type     string `json:"type"` // diamond, bomb
	Revealed bool   `json:"revealed"`
}

// Pre-calculated multiplier lookup tables based on your probability data
var multiplierTable = map[string]map[int]int64{
	"low": { // Easy (σ=0.9, 2 gn) - 2 bombs, 14 diamonds
		0:  1000000,
		1:  1030000,
		2:  1070000,
		3:  1190000,
		4:  1330000,
		5:  1520000,
		6:  1760000,
		7:  2100000,
		8:  2560000,
		9:  3240000,
		10: 4310000,
		11: 6140000,
		12: 9730000,
		13: 18480000,
		14: 52670000,
	},
	"medium": { // Medium (σ=0.9, 4 gn) - 4 bombs, 12 diamonds
		0:  1000000,
		1:  1140000,
		2:  1480000,
		3:  1960000,
		4:  2700000,
		5:  3840000,
		6:  5730000,
		7:  9080000,
		8:  15520000,
		9:  29500000,
		10: 65380000,
		11: 186360000,
		12: 885840000,
	},
	"high": { // Hard (σ=0.9, 6 gn) - 6 bombs, 10 diamonds
		0:  1000000,
		1:  1370000,
		2:  2170000,
		3:  3600000,
		4:  6350000,
		5:  12070000,
		6:  25230000,
		7:  59910000,
		8:  170740000,
		9:  649000000,
		10: 4310910000,
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
func CalculateMultiplier(diamondsFound int, risk string) model.Rate {
	// Get multiplier from lookup table
	if riskTable, exists := multiplierTable[risk]; exists {
		if multiplier, exists := riskTable[diamondsFound]; exists {
			return model.MustRateFromMicro(multiplier)
		}
	}

	// Fallback to 1.0 if not found (should never happen)
	return model.MustRateFromMicro(1000000)
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
func ValidateBetAmount(amount model.Money) bool {
	return amount.MinorUnits() >= 1_00 && amount.MinorUnits() <= 1_000_000_00
}

// CalculatePayoutSafe overflow protection
func CalculatePayoutSafe(betAmount model.Money, multiplier model.Rate) (model.Money, error) {
	return betAmount.Mul(multiplier)
}
