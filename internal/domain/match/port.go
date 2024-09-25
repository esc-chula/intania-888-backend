package match

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
)

type MatchService interface {
	CreateMatch(matchDto *model.MatchDto) error
	GetMatch(matchId string) (*model.MatchDto, error)
	GetAllMatches(filters *model.MatchFilter) ([]*model.MatchDto, error)
	UpdateMatchScore(matchId string, score *model.ScoreDto) error
	UpdateMatchWinner(matchId string, winnerId string) error
	DeleteMatch(id string) error
}

type MatchRepository interface {
	Create(match *model.Match) error
	GetById(matchId string) (*model.Match, error)
	GetAll(filter *model.MatchFilter) ([]*model.Match, error)
	UpdateScore(match *model.Match) error
	UpdateWinner(match *model.Match) error
	Delete(id string) error
}