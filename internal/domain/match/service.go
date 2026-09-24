package match

import (
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/billinglock"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrResultConflict = errors.New("conflicting terminal result")
	ErrInvalidResult  = errors.New("invalid match result")
)

type matchServiceImpl struct {
	repo MatchRepository
	db   *gorm.DB
	log  *zap.Logger
}

func NewMatchService(repo MatchRepository, db *gorm.DB, log *zap.Logger) MatchService {
	return &matchServiceImpl{repo: repo, db: db, log: log}
}

func (s *matchServiceImpl) CreateMatch(d *model.MatchDto) error {
	if d.Id == "" {
		d.Id = uuid.NewString()
	}

	return s.repo.Create(mapMatchDtoToEntity(d))
}

func rateFor(a, b int64, forA bool) (model.Rate, error) {
	if a < 0 || b < 0 {
		return model.Rate{}, model.ErrInvalidRate
	}

	den := new(big.Int).Add(big.NewInt(a), big.NewInt(1))

	if !forA {
		den = new(big.Int).Add(big.NewInt(b), big.NewInt(1))
	}

	total := new(big.Int).Add(big.NewInt(a), big.NewInt(b))
	total.Add(total, big.NewInt(2))

	n := new(big.Int).Mul(total, big.NewInt(1_000_000))
	q, r := new(big.Int), new(big.Int)

	q.QuoRem(n, den, r)

	if new(big.Int).Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	if !q.IsInt64() {
		return model.Rate{}, model.ErrOverflow
	}

	return model.NewRateFromMicro(q.Int64())
}

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

func (s *matchServiceImpl) GetMatch(id string) (*model.MatchDto, error) {
	m, e := s.repo.GetById(id)
	if e != nil {
		return nil, e
	}

	d := mapMatchEntityToDto(m)

	if m.TeamA_Id != nil && m.TeamB_Id != nil {
		a, e := s.repo.CountBetsForTeam(id, *m.TeamA_Id)
		if e != nil {
			return nil, e
		}

		b, e := s.repo.CountBetsForTeam(id, *m.TeamB_Id)
		if e != nil {
			return nil, e
		}
		d.TeamARate, e = rateFor(a, b, true)
		if e != nil {
			return nil, e
		}

		d.TeamBRate, e = rateFor(a, b, false)
		if e != nil {
			return nil, e
		}
	}

	return d, nil
}

func (s *matchServiceImpl) GetAllMatches(f *model.MatchFilter) ([]*model.MatchDto, error) {
	ms, e := s.repo.GetAll(f)
	if e != nil {
		return nil, e
	}

	out := make([]*model.MatchDto, 0, len(ms))

	for _, m := range ms {
		d, e := s.GetMatch(m.Id)
		if e != nil {
			return nil, e
		}

		out = append(out, d)
	}

	return out, nil
}

func (s *matchServiceImpl) UpdateMatchScore(id string, d *model.ScoreDto) error {
	m, e := s.repo.GetById(id)
	if e != nil {
		return e
	}

	m.TeamA_Score = &d.TeamAScore
	m.TeamB_Score = &d.TeamBScore

	return s.repo.UpdateScore(m)
}

func (s *matchServiceImpl) UpdateMatch(id string, d *model.MatchDto) error {
	m, e := s.repo.GetById(id)
	if e != nil {
		return e
	}

	if d.TeamAId != "" {
		m.TeamA_Id = &d.TeamAId
	}

	if d.TeamBId != "" {
		m.TeamB_Id = &d.TeamBId
	}

	if d.TypeId != "" {
		m.TypeId = d.TypeId
	}

	if !d.StartTime.IsZero() {
		m.StartTime = d.StartTime
	}

	if !d.EndTime.IsZero() {
		m.EndTime = d.EndTime
	}

	return s.repo.UpdateMatch(m)
}

func (s *matchServiceImpl) DeleteMatch(id string) error {
	return s.repo.Delete(id)
}

func (s *matchServiceImpl) GetTime() (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

type settlement struct {
	bill   *model.BillHead
	status string
	payout model.Money
}

func (s *matchServiceImpl) SetResult(id string, req *model.MatchResultRequest) error {
	validOutcome := req != nil && (req.Outcome == "winner" || req.Outcome == "draw")
	hasWinner := req != nil && req.WinnerId != nil && *req.WinnerId != ""
	winnerResult := req != nil && req.Outcome == "winner"
	drawResult := req != nil && req.Outcome == "draw"

	if !validOutcome || (winnerResult && !hasWinner) || (drawResult && req.WinnerId != nil) {
		return ErrInvalidResult
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if e := billinglock.Acquire(tx); e != nil {
			return e
		}

		var candidateBillIDs []string

		if e := tx.
			Table("bill_heads").
			Select("DISTINCT bill_heads.id").
			Joins("JOIN bill_lines ON bill_lines.bill_id=bill_heads.id").
			Where("bill_lines.match_id=? AND bill_heads.status='PENDING'", id).
			Order("bill_heads.id").
			Scan(&candidateBillIDs).
			Error; e != nil {
			return e
		}

		matchIDs := []string{id}

		if len(candidateBillIDs) > 0 {
			var referencedMatchIDs []string

			if e := tx.
				Table("bill_lines").
				Select("DISTINCT match_id").
				Where("bill_id IN ?", candidateBillIDs).
				Order("match_id").
				Scan(&referencedMatchIDs).
				Error; e != nil {
				return e
			}

			matchIDs = append(matchIDs, referencedMatchIDs...)
		}

		sort.Strings(matchIDs)
		matchIDs = uniqueStrings(matchIDs)

		var lockedMatches []model.Match

		if e := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", matchIDs).
			Order("id").
			Find(&lockedMatches).
			Error; e != nil {
			return e
		}

		if len(lockedMatches) != len(matchIDs) {
			return gorm.ErrRecordNotFound
		}

		var m model.Match
		found := false

		for i := range lockedMatches {
			if lockedMatches[i].Id != id {
				continue
			}

			m = lockedMatches[i]
			found = true
			break
		}

		if !found {
			return gorm.ErrRecordNotFound
		}

		if m.TeamA_Id == nil || m.TeamB_Id == nil {
			return ErrInvalidResult
		}

		winnerIsKnown := req.Outcome != "winner" || *req.WinnerId == *m.TeamA_Id || *req.WinnerId == *m.TeamB_Id

		if !winnerIsKnown {
			return ErrInvalidResult
		}

		if m.IsDraw || m.WinnerId != nil {
			sameDraw := req.Outcome == "draw" && m.IsDraw
			sameWinner := req.Outcome == "winner" && m.WinnerId != nil && *m.WinnerId == *req.WinnerId
			same := sameDraw || sameWinner

			if same {
				return nil
			}

			return ErrResultConflict
		}

		if req.Outcome == "draw" {
			m.IsDraw = true
			m.WinnerId = nil
		} else {
			m.WinnerId = req.WinnerId
			m.IsDraw = false
		}

		matchUpdates := map[string]any{
			"winner_id":  m.WinnerId,
			"is_draw":    m.IsDraw,
			"updated_at": time.Now(),
		}

		if e := tx.Model(&m).Updates(matchUpdates).Error; e != nil {
			return e
		}

		var ids []string

		if e := tx.
			Table("bill_heads").
			Select("DISTINCT bill_heads.id").
			Joins("JOIN bill_lines ON bill_lines.bill_id=bill_heads.id").
			Where("bill_lines.match_id=? AND bill_heads.status='PENDING'", id).
			Order("bill_heads.id").
			Scan(&ids).
			Error; e != nil {
			return e
		}

		if len(ids) == 0 {
			return nil
		}

		var bills []*model.BillHead

		if e := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", ids).
			Order("id").
			Find(&bills).
			Error; e != nil {
			return e
		}

		var lines []model.BillLine

		if e := tx.
			Preload("Match").
			Where("bill_id IN ?", ids).
			Order("bill_id, match_id").
			Find(&lines).
			Error; e != nil {
			return e
		}

		byBill := map[string][]model.BillLine{}

		for _, l := range lines {
			if l.MatchId == id {
				l.Match = m
			}

			byBill[l.BillId] = append(byBill[l.BillId], l)
		}

		sets := make([]settlement, 0)
		userSet := map[string]bool{}

		for _, b := range bills {
			if b.Status != "PENDING" {
				continue
			}

			all := true
			lost := false
			rates := []model.Rate{}

			for _, l := range byBill[b.Id] {
				lm := l.Match

				if lm.IsDraw {
					continue
				}

				if lm.WinnerId == nil {
					all = false
					continue
				}

				if *lm.WinnerId != l.BettingOn {
					lost = true
					break
				}

				rates = append(rates, model.MustRateFromMicro(l.Rate))
			}

			if lost {
				sets = append(sets, settlement{
					bill:   b,
					status: "LOST",
					payout: model.MustMoneyFromMinor(0),
				})
				userSet[b.UserId] = true
			} else if all {
				p, e := model.AccumulatorPayout(model.MustMoneyFromMinor(b.Total), rates)
				if e != nil {
					return e
				}

				sets = append(sets, settlement{
					bill:   b,
					status: "WON",
					payout: p,
				})
				userSet[b.UserId] = true
			}
		}

		uids := make([]string, 0, len(userSet))

		for u := range userSet {
			uids = append(uids, u)
		}

		sort.Strings(uids)

		var users []model.User

		if len(uids) > 0 {
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", uids).Order("id").Find(&users).Error; e != nil {
				return e
			}
		}

		um := map[string]*model.User{}

		for i := range users {
			um[users[i].Id] = &users[i]
		}

		now := time.Now()

		for _, st := range sets {
			p := st.payout.MinorUnits()

			if st.status == "WON" && !st.payout.IsZero() {
				u := um[st.bill.UserId]
				nb, e := model.MustMoneyFromMinor(u.RemainingCoin).Add(st.payout)

				if e != nil {
					return e
				}

				if e = tx.Model(u).Update("remaining_coin", nb.MinorUnits()).Error; e != nil {
					return e
				}

				u.RemainingCoin = nb.MinorUnits()
			}

			billUpdates := map[string]any{
				"status":     st.status,
				"payout":     p,
				"settled_at": now,
				"updated_at": now,
			}

			if e := tx.Model(st.bill).Updates(billUpdates).Error; e != nil {
				return e
			}

			ev := model.BillTerminalEvent{
				Id:        uuid.NewString(),
				BillId:    st.bill.Id,
				Kind:      "SETTLED",
				Amount:    p,
				CreatedAt: now,
			}

			if e := tx.Create(&ev).Error; e != nil {
				return e
			}
		}

		return nil
	})
}
