package match

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
)

type MatchService interface {
	CreateMatch(userProfile *model.UserDto, matchDto *model.MatchDto) error
	GetMatch(matchId, userId string) (*model.MatchDto, error)
	GetAllMatches(userId string) ([]*model.MatchDto, error)
	UpdateMatch(matchDto *model.MatchDto) error
	DeleteMatch(id string) error
}

type MatchRepository interface {
	Create(match *model.Match) error
	GetById(matchId, userId string) (*model.Match, error)
	GetAll(userId string) ([]*model.Match, error)
	Update(match *model.Match) error
	Delete(id string) error
}
