package match

import (
	"errors"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"go.uber.org/zap"
)

type matchServiceImpl struct {
	repo MatchRepository
	log  *zap.Logger
}

func NewMatchService(repo MatchRepository, log *zap.Logger) MatchService {
	return &matchServiceImpl{repo, log}
}

func (s *matchServiceImpl) CreateMatch(matchDto *model.MatchDto) error {
	match := mapMatchDtoToEntity(matchDto)
	err := s.repo.Create(match)
	if err != nil {
		s.log.Named("CreateMatch").Error("Create", zap.Error(err))
		return err
	}

	s.log.Named("CreateMatch").Info("Created match successful", zap.Any("match", match))
	return nil
}

func (s *matchServiceImpl) GetMatch(matchId string) (*model.MatchDto, error) {
	match, err := s.repo.GetById(matchId)
	if err != nil {
		s.log.Named("GetMatch").Error("GetById", zap.Error(err))
		return nil, err
	}

	if match == nil {
		s.log.Named("GetMatch").Error("Match not found", zap.String("id", matchId))
		return nil, errors.New("match not found")
	}

	// Count the bet for each team
	teamACount, err := s.repo.CountBetsForTeam(matchId, *match.TeamA_Id)
	if err != nil {
		s.log.Named("GetMatch").Error("CountBetsForTeam", zap.Error(err))
		return nil, err
	}
	teamBCount, err := s.repo.CountBetsForTeam(matchId, *match.TeamB_Id)
	if err != nil {
		s.log.Named("GetMatch").Error("CountBetsForTeam", zap.Error(err))
		return nil, err
	}

	// Calculate the odds rate for each team
	rateA := calculateOddsRate("A", float64(teamACount), float64(teamBCount))
	rateB := calculateOddsRate("B", float64(teamACount), float64(teamBCount))

	matchDto := mapMatchEntityToDto(match)
	matchDto.TeamARate = rateA
	matchDto.TeamBRate = rateB
	s.log.Named("GetMatch").Info("Retrieved match successful", zap.String("id", matchId))
	return matchDto, nil
}

func (s *matchServiceImpl) GetAllMatches(filter *model.MatchFilter) ([]*model.MatchDto, error) {
	matches, err := s.repo.GetAll(filter)
	if err != nil {
		s.log.Named("GetAllMatches").Error("Failed to fetch matches", zap.Error(err))
		return nil, err
	}

	matchesDto := make([]*model.MatchDto, len(matches))
	for i, match := range matches {
		// Count bets for both teams
		teamACount, err := s.repo.CountBetsForTeam(match.Id, *match.TeamA_Id)
		if err != nil {
			return nil, err
		}

		teamBCount, err := s.repo.CountBetsForTeam(match.Id, *match.TeamB_Id)
		if err != nil {
			return nil, err
		}

		// Calculate odds
		rateA := calculateOddsRate("A", float64(teamACount), float64(teamBCount))
		rateB := calculateOddsRate("B", float64(teamACount), float64(teamBCount))

		matchDto := mapMatchEntityToDto(match)
		matchDto.TeamARate = rateA
		matchDto.TeamBRate = rateB

		matchesDto[i] = matchDto
	}
	s.log.Named("GetAllMatches").Info("Retrieved all matches successfully")
	return matchesDto, nil
}

func (s *matchServiceImpl) UpdateMatchScore(matchId string, scoreDto *model.ScoreDto) error {
	existingMatch, err := s.repo.GetById(matchId)
	if err != nil {
		s.log.Named("UpdateMatchScore").Error("GetById", zap.Error(err))
		return err
	}

	if existingMatch == nil {
		s.log.Named("UpdateMatchScore").Error("Match not found", zap.String("id", matchId))
		return errors.New("match not found")
	}

	existingMatch.TeamA_Score = &scoreDto.TeamAScore
	existingMatch.TeamB_Score = &scoreDto.TeamBScore

	err = s.repo.UpdateScore(existingMatch)
	if err != nil {
		s.log.Named("UpdateMatchScore").Error("UpdateScore", zap.Error(err))
		return err
	}

	s.log.Named("UpdateMatchScore").Info("Updated match score successfully", zap.String("id", matchId))
	return nil
}

func (s *matchServiceImpl) UpdateMatchWinner(matchId string, winnerId string) error {
	existingMatch, err := s.repo.GetById(matchId)
	if err != nil {
		s.log.Named("UpdateMatchWinner").Error("GetById", zap.Error(err))
		return err
	}

	if existingMatch == nil {
		s.log.Named("UpdateMatchWinner").Error("Match not found", zap.String("id", matchId))
		return errors.New("match not found")
	}

	existingMatch.WinnerId = &winnerId

	err = s.repo.UpdateWinner(existingMatch)
	if err != nil {
		s.log.Named("UpdateMatchWinner").Error("UpdateWinner", zap.Error(err))
		return err
	}

	s.log.Named("UpdateMatchWinner").Info("Updated match winner successfully", zap.String("id", matchId))
	return nil
}

func (s *matchServiceImpl) DeleteMatch(id string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.Named("DeleteMatch").Error("Delete", zap.Error(err))
		return err
	}

	s.log.Named("DeleteMatch").Info("Deleted match successful", zap.String("id", id))
	return nil
}
