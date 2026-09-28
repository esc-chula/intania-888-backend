package event

import (
	"math/rand/v2"

	"github.com/esc-chula/intania-888-backend/internal/identity"
)

// TODO(DR-005): review probability mass and map traversal in the slot-game sprint.
// Preserve the inherited weights, traversal, and fallback during normalization.
func getRandomSlot(actor identity.Profile) string {
	var probabilities map[string]float64

	if actor.RemainingCoin.MinorUnits() > 100_000_00 {
		probabilities = map[string]float64{
			"🍇": 1.0 / 7.0,
			"🍋": 1.0 / 7.0,
			"🍎": 1.0 / 7.0,
			"🍐": 1.0 / 7.0,
			"🍊": 1.0 / 7.0,
			"💰": 1.0 / 20.0,
			"👽": 1.0 / 7.0,
		}
	} else if actor.RemainingCoin.MinorUnits() > 50_000_00 {
		probabilities = map[string]float64{
			"🍇": 1.0 / 7.0,
			"🍋": 1.0 / 7.0,
			"🍎": 1.0 / 7.0,
			"🍐": 1.0 / 7.0,
			"🍊": 1.0 / 7.0,
			"💰": 1.0 / 10.0,
			"👽": 1.0 / 7.0,
		}
	} else if actor.RemainingCoin.MinorUnits() > 25_000_00 {
		probabilities = map[string]float64{
			"🍇": 1.0 / 7.0,
			"🍋": 1.0 / 7.0,
			"🍎": 1.0 / 7.0,
			"🍐": 1.0 / 7.0,
			"🍊": 1.0 / 7.0,
			"💰": 1.0 / 8.0,
			"👽": 1.0 / 7.0,
		}
	} else {
		probabilities = map[string]float64{
			"🍇": 1.0 / 7.0,
			"🍋": 1.0 / 7.0,
			"🍎": 1.0 / 7.0,
			"🍐": 1.0 / 7.0,
			"🍊": 1.0 / 7.0,
			"💰": 1.0 / 6.0,
			"👽": 1.0 / 7.0,
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
