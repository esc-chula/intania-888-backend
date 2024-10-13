package match

import (
	"errors"
	"time"

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

// Helper method to fetch match by ID
func (s *matchServiceImpl) getMatchById(matchId string) (*model.Match, error) {
	match, err := s.repo.GetById(matchId)
	if err != nil {
		s.log.Error("Failed to fetch match by ID", zap.Error(err))
		return nil, err
	}

	if match == nil {
		s.log.Warn("Match not found", zap.String("match_id", matchId))
		return nil, errors.New("match not found")
	}

	return match, nil
}

func (s *matchServiceImpl) UpdateMatchWinner(matchId string, winnerId string) error {
	// Fetch the match by ID
	existingMatch, err := s.getMatchById(matchId)
	if err != nil {
		return err
	}

	// Set the match winner
	if err := s.setMatchWinner(existingMatch, winnerId); err != nil {
		return err
	}

	// Process payouts for users who bet on the winner
	if err := s.processPayoutsForMatch(matchId); err != nil {
		return err
	}

	s.log.Info("Updated match winner and processed payouts", zap.String("match_id", matchId))
	return nil
}

func (s *matchServiceImpl) setMatchWinner(match *model.Match, winnerId string) error {
	match.WinnerId = &winnerId
	err := s.repo.UpdateWinner(match)
	if err != nil {
		s.log.Error("Failed to update match winner", zap.Error(err))
		return err
	}
	return nil
}

func (s *matchServiceImpl) processPayoutsForMatch(matchId string) error {
	// Fetch all bill heads associated with the match
	billHeads, err := s.repo.GetBillHeadsForMatch(matchId)
	if err != nil {
		s.log.Error("Failed to fetch bill heads for match", zap.Error(err))
		return err
	}

	// Process each bill head
	for _, billHead := range billHeads {
		allLinesResolved := true
		var totalRates float64 = 1.0

		for _, billLine := range billHead.Lines {
			match, err := s.repo.GetById(billLine.MatchId)
			if err != nil {
				return err
			}

			// Skip already paid lines
			if billLine.IsPaid {
				s.log.Info("Skipping already paid bill line", zap.String("bill_id", billLine.BillId))
				continue
			}

			// Check if the match is a draw
			if match.IsDraw {
				totalRates *= 1 // Use a rate of 1 for draw matches
			} else if match.WinnerId == nil {
				allLinesResolved = false
				break
			} else if match.WinnerId != nil && *match.WinnerId == billLine.BettingOn {
				totalRates *= billLine.Rate // Normal rate for winning bets
			} else {
				totalRates *= 0
			}
		}

		// If all bill lines are resolved, calculate the payout
		if allLinesResolved {
			payout := calculatePayout(totalRates, billHead.Total)
			err := s.repo.PayoutToUser(billHead.UserId, payout)
			if err != nil {
				s.log.Error("Failed to process payout for user", zap.Error(err))
				return err
			}

			// Mark all bill lines as paid
			for _, billLine := range billHead.Lines {
				err = s.repo.MarkBillLineAsPaid(billLine.BillId, billLine.MatchId)
				if err != nil {
					s.log.Error("Failed to mark bill line as paid", zap.Error(err))
					return err
				}
			}

			s.log.Info("Processed payout for user", zap.String("user_id", billHead.UserId))
		}
	}

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

func (s *matchServiceImpl) GetTime() (string, error) {
	return time.Now().Format("2006-01-02 15:04:05"), nil
}

func (s *matchServiceImpl) UpdateMatchDraw(matchId string) error {
	// Fetch the match by ID
	match, err := s.getMatchById(matchId)
	if err != nil {
		return err
	}

	// Set the match as a draw
	match.IsDraw = true
	err = s.repo.UpdateMatch(match)
	if err != nil {
		s.log.Error("Failed to update match as draw", zap.Error(err))
		return err
	}

	// Process payouts with draw logic
	err = s.processPayoutsForMatch(matchId)
	if err != nil {
		s.log.Error("Failed to process payouts for draw match", zap.Error(err))
		return err
	}

	s.log.Info("Updated match as draw and processed payouts", zap.String("match_id", matchId))
	return nil
}
