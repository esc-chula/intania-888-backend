package utils

import (
	"math/rand/v2"

	"github.com/esc-chula/intania-888-backend/internal/model"
)

func GetRandomSlot(userDto *model.UserDto) string {
	var probabilities map[string]float64

	if userDto.RemainingCoin > 50000.00 {
		probabilities = map[string]float64{
			"🍇": 1.0 / 5.8,
			"🍋": 1.0 / 5.8,
			"🍎": 1.0 / 5.8,
			"🍐": 1.0 / 5.8,
			"🍊": 1.0 / 5.8,
			"💰": 1.0 / 30.0,
		}
	} else {
		probabilities = map[string]float64{
			"🍇": 1.0 / 6.0,
			"🍋": 1.0 / 6.0,
			"🍎": 1.0 / 6.0,
			"🍐": 1.0 / 6.0,
			"🍊": 1.0 / 6.0,
			"💰": 1.0 / 6.0,
		}
	}

	random := rand.Float64()
	var cumulative float64

	for symbol, probability := range probabilities {
		cumulative += probability
		if random < cumulative {
			return symbol
		}
	}
	return "💰"
}
