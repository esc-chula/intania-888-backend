package match

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
)

type MatchService interface {
	CreateMatch(matchDto *model.MatchDto) error
	GetMatch(matchId string) (*model.MatchDto, error)
	GetTime() (string, error)
	GetAllMatches(filters *model.MatchFilter) ([]*model.MatchDto, error)
	UpdateMatchScore(matchId string, score *model.ScoreDto) error
	UpdateMatchWinner(matchId string, winnerId string) error
	processPayoutsForMatch(matchId string) error
	UpdateMatchDraw(matchId string) error
	DeleteMatch(id string) error
}

type MatchRepository interface {
	Create(match *model.Match) error
	GetById(matchId string) (*model.Match, error)
	GetAll(filter *model.MatchFilter) ([]*model.Match, error)
	CountBetsForTeam(matchId string, teamId string) (int64, error)
	UpdateScore(match *model.Match) error
	UpdateWinner(match *model.Match) error
	UpdateMatch(match *model.Match) error
	GetBillHeadsForMatch(matchId string) ([]*model.BillHead, error)
	MarkBillLineAsPaid(billLineId string, matchId string) error
	PayoutToUser(userId string, amount float64) error
	Delete(id string) error
}
