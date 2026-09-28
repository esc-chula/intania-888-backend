package bill

import (
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/billinglock"
	"github.com/esc-chula/intania-888-backend/internal/domain/user"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidBill         = errors.New("invalid bill")
	ErrMatchNotFound       = errors.New("match not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrBillConflict        = errors.New("bill lifecycle conflict")
)

type billServiceImpl struct {
	repo BillRepository
	db   *gorm.DB
	log  *zap.Logger
}

func NewBillService(repo BillRepository, _ user.UserRepository, db *gorm.DB, log *zap.Logger) BillService {
	return &billServiceImpl{repo: repo, db: db, log: log}
}

func (s *billServiceImpl) CreateBill(userID string, req *model.CreateBillRequest) (*model.BillHeadDto, error) {
	if req == nil || req.Total.IsZero() || len(req.Lines) == 0 {
		return nil, ErrInvalidBill
	}

	lines := append([]model.CreateBillLineRequest(nil), req.Lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].MatchId < lines[j].MatchId })

	for i, l := range lines {
		matchIDEmpty := strings.TrimSpace(l.MatchId) == ""
		bettingOnEmpty := strings.TrimSpace(l.BettingOn) == ""
		duplicateMatch := i > 0 && l.MatchId == lines[i-1].MatchId

		if matchIDEmpty || bettingOnEmpty || duplicateMatch {
			return nil, ErrInvalidBill
		}
	}

	var made model.BillHead
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := billinglock.Acquire(tx); err != nil {
			return err
		}

		ids := make([]string, len(lines))
		for i := range lines {
			ids[i] = lines[i].MatchId
		}

		var ms []model.Match

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&ms).Error; err != nil {
			return err
		}

		if len(ms) != len(lines) {
			return ErrMatchNotFound
		}

		byID := map[string]model.Match{}
		for _, m := range ms {
			byID[m.Id] = m
		}

		made = model.BillHead{
			Id:        uuid.NewString(),
			Total:     req.Total.MinorUnits(),
			UserId:    userID,
			Status:    "PENDING",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		rates := make([]model.Rate, 0, len(lines))
		now := time.Now()

		for _, in := range lines {
			m := byID[in.MatchId]
			missingTeams := m.TeamA_Id == nil || m.TeamB_Id == nil
			invalidSelection := !missingTeams && in.BettingOn != *m.TeamA_Id && in.BettingOn != *m.TeamB_Id
			started := !now.Before(m.StartTime)

			if missingTeams || invalidSelection || started || m.WinnerId != nil || m.IsDraw {
				return ErrInvalidBill
			}

			var cs []struct {
				BettingOn string
				Count     int64
			}
			if err := tx.
				Table("bill_lines").
				Select("bill_lines.betting_on, count(*) AS count").
				Joins("JOIN bill_heads ON bill_heads.id = bill_lines.bill_id").
				Where("bill_lines.match_id = ? AND bill_heads.status = 'PENDING'", m.Id).
				Group("bill_lines.betting_on").
				Scan(&cs).
				Error; err != nil {
				return err
			}

			var a, b int64

			for _, c := range cs {
				if c.BettingOn == *m.TeamA_Id {
					a = c.Count
				}

				if c.BettingOn == *m.TeamB_Id {
					b = c.Count
				}
			}

			rate, err := seededRate(a, b, in.BettingOn == *m.TeamA_Id)

			if err != nil {
				return err
			}

			rates = append(rates, rate)

			made.Lines = append(made.Lines, model.BillLine{
				BillId:    made.Id,
				MatchId:   m.Id,
				BettingOn: in.BettingOn,
				Rate:      rate.MicroUnits(),
				Match:     m,
			})
		}

		if _, err := model.AccumulatorPayout(req.Total, rates); err != nil {
			return err
		}

		var u model.User

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", userID).Error; err != nil {
			return err
		}

		if u.RemainingCoin < req.Total.MinorUnits() {
			return ErrInsufficientBalance
		}

		if err := tx.Omit("Lines.Match").Create(&made).Error; err != nil {
			return err
		}

		r := tx.Model(&model.User{}).Where("id = ? AND remaining_coin >= ?", userID, made.Total).Update("remaining_coin", gorm.Expr("remaining_coin - ?", made.Total))

		if r.Error != nil {
			return r.Error
		}

		if r.RowsAffected != 1 {
			return ErrInsufficientBalance
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return mapBillEntityToDto(&made), nil
}

func seededRate(a, b int64, forA bool) (model.Rate, error) {
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

func (s *billServiceImpl) GetBill(id, uid string) (*model.BillHeadDto, error) {
	v, e := s.repo.GetById(id, uid)

	if e != nil {
		return nil, e
	}

	return mapBillEntityToDto(v), nil
}

func (s *billServiceImpl) GetAllBills(uid string) ([]*model.BillHeadDto, error) {
	v, e := s.repo.GetAll(uid)

	if e != nil {
		return nil, e
	}

	return mapBillsEntityToDto(v), nil
}

func (s *billServiceImpl) GetAllBillsAdmin() ([]*model.BillHeadDto, error) {
	v, e := s.repo.GetAllAdmin()

	if e != nil {
		return nil, e
	}

	return mapBillsEntityToDto(v), nil
}

func (s *billServiceImpl) VoidBill(id, actor, reason string) (*model.BillHeadDto, error) {
	actor = strings.TrimSpace(actor)
	reason = strings.TrimSpace(reason)
	if actor == "" || len(reason) == 0 || len(reason) > 500 {
		return nil, ErrInvalidBill
	}

	var v model.BillHead
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if e := billinglock.Acquire(tx); e != nil {
			return e
		}

		var matchIDs []string

		if e := tx.
			Table("bill_lines").
			Select("match_id").
			Where("bill_id = ?", id).
			Order("match_id").
			Scan(&matchIDs).
			Error; e != nil {
			return e
		}

		if len(matchIDs) > 0 {
			var matches []model.Match

			if e := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id IN ?", matchIDs).
				Order("id").
				Find(&matches).
				Error; e != nil {
				return e
			}

			if len(matches) != len(matchIDs) {
				return gorm.ErrRecordNotFound
			}
		}

		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Lines").Preload("Lines.Match").First(&v, "id = ?", id).Error; e != nil {
			return e
		}

		if v.Status == "VOIDED" {
			return nil
		}

		if v.Status != "PENDING" {
			return ErrBillConflict
		}

		var u model.User

		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, "id = ?", v.UserId).Error; e != nil {
			return e
		}

		balance, e := model.MustMoneyFromMinor(u.RemainingCoin).Add(model.MustMoneyFromMinor(v.Total))

		if e != nil {
			return e
		}

		now := time.Now()
		p := v.Total
		updates := map[string]any{
			"status":     "VOIDED",
			"payout":     p,
			"voided_at":  now,
			"updated_at": now,
		}

		if e = tx.Model(&v).Updates(updates).Error; e != nil {
			return e
		}

		if e = tx.Model(&u).Update("remaining_coin", balance.MinorUnits()).Error; e != nil {
			return e
		}

		ev := model.BillTerminalEvent{
			Id:        uuid.NewString(),
			BillId:    v.Id,
			Kind:      "VOIDED",
			Amount:    v.Total,
			ActorId:   &actor,
			Reason:    &reason,
			CreatedAt: now,
		}

		if e = tx.Create(&ev).Error; e != nil {
			return e
		}

		v.Status = "VOIDED"
		v.Payout = &p
		v.VoidedAt = &now

		return nil
	})

	if err != nil {
		return nil, err
	}

	return mapBillEntityToDto(&v), nil
}
