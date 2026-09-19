package match

import "github.com/esc-chula/intania-888-backend/internal/model"

type MatchService interface {
	CreateMatch(*model.MatchDto) error
	GetMatch(string) (*model.MatchDto, error)
	GetTime() (string, error)
	GetAllMatches(*model.MatchFilter) ([]*model.MatchDto, error)
	UpdateMatchScore(string, *model.ScoreDto) error
	SetResult(string, *model.MatchResultRequest) error
	UpdateMatch(string, *model.MatchDto) error
	DeleteMatch(string) error
}

type MatchRepository interface {
	Create(*model.Match) error
	GetById(string) (*model.Match, error)
	GetAll(*model.MatchFilter) ([]*model.Match, error)
	CountBetsForTeam(string, string) (int64, error)
	UpdateScore(*model.Match) error
	UpdateMatch(*model.Match) error
	Delete(string) error
}
