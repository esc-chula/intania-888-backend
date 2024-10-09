package utils

import "math/rand/v2"

func GetRandomSlot() string {
	probabilities := map[string]float64{
		"🍇": 141.0 / 216.0,
		"🍋": 141.0 / 216.0,
		"🍎": 141.0 / 216.0,
		"🍐": 141.0 / 216.0,
		"🍊": 141.0 / 216.0,
		"💰": 141.0 / 216.0,
	}
	random := rand.Float64()
	var cumulative float64

	for symbol, probability := range probabilities {
		cumulative += probability
		if random < cumulative {
			return symbol
		}
	}
	return "🍇"
}
