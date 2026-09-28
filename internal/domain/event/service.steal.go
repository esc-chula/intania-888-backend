package event

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/esc-chula/intania-888-backend/internal/identity"
	"github.com/esc-chula/intania-888-backend/internal/value"
)

// UseStealToken validates and consumes a token in the same transaction as its balances.
// The minimum thief credit remains 50.00 even when the victim debit is smaller.
func (s *Service) UseStealToken(ctx context.Context, userID, tokenValue string, victimIndex int) (*StealResult, error) {
	var result *StealResult
	err := s.repo.WithinTransaction(ctx, func(tx TransactionRepository) error {
		token, err := tx.LockStealToken(ctx, tokenValue)
		if err != nil {
			return err
		}
		if token.UserID != userID {
			return ErrStealTokenForbidden
		}
		if token.IsUsed {
			return ErrStealTokenConflict
		}
		if s.now().After(token.ExpiresAt) {
			return ErrStealTokenInvalid
		}
		if victimIndex < 0 || victimIndex >= len(token.AllowedVictimIDs) {
			return ErrInvalidStealRequest
		}
		chosenID := token.AllowedVictimIDs[victimIndex]
		if chosenID == userID {
			return errors.New("cannot steal from yourself")
		}

		// Preserve token-first locking and sorted account locks for reciprocal raids.
		lockIDs := []string{userID, chosenID}
		sort.Strings(lockIDs)
		locked := make(map[string]identity.User, len(lockIDs))
		for _, id := range lockIDs {
			actor, err := tx.LockUser(ctx, id)
			if err != nil {
				return err
			}
			locked[id] = actor
		}
		victim := locked[chosenID]
		thief := locked[userID]
		if victim.RemainingCoin < minStealVictimBalanceMinor {
			return ErrInsufficientBalance
		}

		victimBefore, err := value.NewMoneyFromMinor(victim.RemainingCoin)
		if err != nil {
			return err
		}
		stolen, err := victimBefore.Mul(value.MustRateFromMicro(stealPercentageMicro))
		if err != nil {
			return err
		}
		if stolen.IsZero() {
			return errors.New("calculated steal is zero")
		}

		// Snapshot candidates before the balance writes for the established response.
		candidates, err := tx.GetUsersByIDs(ctx, token.AllowedVictimIDs)
		if err != nil {
			return err
		}
		credit := stolen
		if stolen.MinorUnits() < minStealAmountMinor {
			credit = value.MustMoneyFromMinor(minStealAmountMinor)
		}
		victimBalance, err := victimBefore.Sub(stolen)
		if err != nil {
			return err
		}
		thiefBefore, err := value.NewMoneyFromMinor(thief.RemainingCoin)
		if err != nil {
			return err
		}
		thiefBalance, err := thiefBefore.Add(credit)
		if err != nil {
			return err
		}

		if err := tx.SetUserBalance(ctx, victim.ID, victimBalance); err != nil {
			return err
		}
		if err := tx.SetUserBalance(ctx, thief.ID, thiefBalance); err != nil {
			return err
		}
		used, err := tx.MarkTokenUsed(ctx, token.ID)
		if err != nil {
			return err
		}
		if !used {
			return ErrStealTokenConflict
		}

		result, err = buildStealResult(token.AllowedVictimIDs, candidates, chosenID, credit, thiefBalance)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func buildStealResult(candidateIDs []string, candidates []identity.User, chosenID string, credit, thiefBalance value.Money) (*StealResult, error) {
	byID := make(map[string]identity.User, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	details := make([]VictimDetail, 0, 3)
	for i, id := range candidateIDs {
		candidate, found := byID[id]
		if !found {
			details = append(details, VictimDetail{Index: i, Name: "[Deleted User]", RoleID: "UNKNOWN", WasChosen: id == chosenID})
			continue
		}
		balance, err := value.NewMoneyFromMinor(candidate.RemainingCoin)
		if err != nil {
			return nil, err
		}
		detail := VictimDetail{Index: i, UserID: candidate.ID, Name: candidate.Name, RoleID: candidate.RoleID,
			GroupID: candidate.GroupID, BalanceBefore: balance, WasChosen: id == chosenID}
		if detail.WasChosen {
			detail.AmountStolen = credit
		}
		details = append(details, detail)
	}
	chosen, exists := byID[chosenID]
	if !exists {
		return nil, errors.New("chosen victim no longer exists")
	}
	return &StealResult{TotalStolen: credit, RaiderNewBalance: thiefBalance, AllCandidates: details,
		Message: fmt.Sprintf("👽 You raided %s and stole %s coins!", chosen.Name, credit.String())}, nil
}
