package match

import (
	"context"
	"sort"

	"github.com/esc-chula/intania-888-backend/internal/value"
)

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}

	unique := values[:1]

	for _, value := range values[1:] {
		if value == unique[len(unique)-1] {
			continue
		}

		unique = append(unique, value)
	}

	return unique
}

type settlement struct {
	bill   *BillSnapshot
	status string
	payout value.Money
}

// SetResult records a terminal result and settles eligible bills atomically.
// An identical repeated result succeeds without another payment or audit event.
func (s *Service) SetResult(ctx context.Context, id string, req *ResultInput) error {
	validOutcome := req != nil && (req.Outcome == "winner" || req.Outcome == "draw")
	hasWinner := req != nil && req.WinnerID != nil && *req.WinnerID != ""
	winnerResult := req != nil && req.Outcome == "winner"
	drawResult := req != nil && req.Outcome == "draw"
	if !validOutcome || (winnerResult && !hasWinner) || (drawResult && req.WinnerID != nil) {
		return ErrInvalidResult
	}
	return s.transactions.WithinTransaction(ctx, func(tx TransactionRepository) error {
		if err := tx.AcquireLifecycleLock(ctx); err != nil {
			return err
		}
		candidateBillIDs, err := tx.FindPendingBillIDs(ctx, id)
		if err != nil {
			return err
		}

		matchIDs := []string{id}
		if len(candidateBillIDs) > 0 {
			referenced, err := tx.FindReferencedMatchIDs(ctx, candidateBillIDs)
			if err != nil {
				return err
			}
			matchIDs = append(matchIDs, referenced...)
		}

		sort.Strings(matchIDs)
		matchIDs = uniqueStrings(matchIDs)
		matches, err := tx.LockMatches(ctx, matchIDs)
		if err != nil {
			return err
		}
		if len(matches) != len(matchIDs) {
			return ErrNotFound
		}

		var current Snapshot
		found := false
		for i := range matches {
			if matches[i].ID != id {
				continue
			}
			current = matches[i]
			found = true
			break
		}
		if !found {
			return ErrNotFound
		}

		if current.TeamAID == nil || current.TeamBID == nil {
			return ErrInvalidResult
		}

		winnerIsKnown := req.Outcome != "winner" || *req.WinnerID == *current.TeamAID || *req.WinnerID == *current.TeamBID
		if !winnerIsKnown {
			return ErrInvalidResult
		}

		if current.IsDraw || current.WinnerID != nil {
			sameDraw := req.Outcome == "draw" && current.IsDraw
			sameWinner := req.Outcome == "winner" && current.WinnerID != nil && *current.WinnerID == *req.WinnerID
			if sameDraw || sameWinner {
				return nil
			}
			return ErrResultConflict
		}
		if req.Outcome == "draw" {
			current.IsDraw = true
			current.WinnerID = nil
		} else {
			current.WinnerID = req.WinnerID
			current.IsDraw = false
		}
		if err := tx.UpdateResult(ctx, &current, s.now()); err != nil {
			return err
		}

		ids, err := tx.FindPendingBillIDs(ctx, id)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		bills, err := tx.LockBills(ctx, ids)
		if err != nil {
			return err
		}

		lines, err := tx.FindBillLines(ctx, ids)
		if err != nil {
			return err
		}

		byBill := make(map[string][]BillLineSnapshot)
		for _, line := range lines {
			if line.MatchID == id {
				line.Match = current
			}
			byBill[line.BillID] = append(byBill[line.BillID], line)
		}

		settlements := make([]settlement, 0)
		userSet := make(map[string]bool)
		for _, bill := range bills {
			if bill.Status != "PENDING" {
				continue
			}
			all := true
			lost := false
			rates := []value.Rate{}
			for _, line := range byBill[bill.ID] {
				item := line.Match
				if item.IsDraw {
					continue
				}
				if item.WinnerID == nil {
					all = false
					continue
				}
				if *item.WinnerID != line.BettingOn {
					lost = true
					break
				}
				rates = append(rates, line.Rate)
			}
			if lost {
				settlements = append(settlements, settlement{
					bill:   bill,
					status: "LOST",
					payout: value.MustMoneyFromMinor(0),
				})
				userSet[bill.UserID] = true
			} else if all {
				payout, err := value.AccumulatorPayout(bill.Total, rates)
				if err != nil {
					return err
				}
				settlements = append(settlements, settlement{bill: bill, status: "WON", payout: payout})
				userSet[bill.UserID] = true
			}
		}

		userIDs := make([]string, 0, len(userSet))
		for userID := range userSet {
			userIDs = append(userIDs, userID)
		}
		sort.Strings(userIDs)

		users := []UserBalance{}
		if len(userIDs) > 0 {
			users, err = tx.LockUsers(ctx, userIDs)
			if err != nil {
				return err
			}
		}

		byUser := make(map[string]*UserBalance, len(users))
		for i := range users {
			byUser[users[i].ID] = &users[i]
		}

		now := s.now()
		for _, item := range settlements {
			if item.status == "WON" && !item.payout.IsZero() {
				user := byUser[item.bill.UserID]
				balance, err := user.Balance.Add(item.payout)
				if err != nil {
					return err
				}
				if err := tx.UpdateBalance(ctx, user.ID, balance); err != nil {
					return err
				}
				user.Balance = balance
			}
			update := BillSettlement{BillID: item.bill.ID, Status: item.status, Payout: item.payout, SettledAt: now}
			if err := tx.SettleBill(ctx, update); err != nil {
				return err
			}
			event := TerminalEvent{ID: s.newID(), BillID: item.bill.ID, Amount: item.payout, CreatedAt: now}
			if err := tx.CreateTerminalEvent(ctx, event); err != nil {
				return err
			}
		}
		return nil
	})
}
