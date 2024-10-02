package match

import (
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
)

func mapMatchDtoToEntity(matchDto *model.MatchDto) *model.Match {
	return &model.Match{
		Id: matchDto.Id,
		TeamA_Id: func() *string {
			if matchDto.TeamAId != "" {
				return &matchDto.TeamAId
			}
			return nil
		}(),
		TeamB_Id: func() *string {
			if matchDto.TeamBId != "" {
				return &matchDto.TeamBId
			}
			return nil
		}(),
		TeamA_Score: matchDto.TeamAScore,
		TeamB_Score: matchDto.TeamBScore,
		WinnerId: func() *string {
			if matchDto.WinnerId != "" {
				return &matchDto.WinnerId
			}
			return nil
		}(),
		TypeId:    matchDto.TypeId,
		StartTime: matchDto.StartTime,
		EndTime:   matchDto.EndTime,
	}
}

func mapMatchEntityToDto(match *model.Match) *model.MatchDto {
	return &model.MatchDto{
		Id: match.Id,
		TeamAId: func() string {
			if match.TeamA_Id != nil {
				return *match.TeamA_Id
			}
			return ""
		}(),
		TeamBId: func() string {
			if match.TeamB_Id != nil {
				return *match.TeamB_Id
			}
			return ""
		}(),
		TeamAScore: match.TeamA_Score,
		TeamBScore: match.TeamB_Score,
		WinnerId: func() string {
			if match.WinnerId != nil {
				return *match.WinnerId
			}
			return ""
		}(),
		TypeId:    match.TypeId,
		StartTime: match.StartTime,
		EndTime:   match.EndTime,
	}
}

func groupMatchesByDateAndType(matches []*model.MatchDto) []model.MatchesByDate {
	dateMap := make(map[time.Time]map[string][]*model.MatchDto)

	for _, match := range matches {
		date := match.StartTime.Truncate(24 * time.Hour)
		sportType := match.TypeId

		if _, dateExists := dateMap[date]; !dateExists {
			dateMap[date] = make(map[string][]*model.MatchDto)
		}

		dateMap[date][sportType] = append(dateMap[date][sportType], match)
	}

	var response []model.MatchesByDate

	for date, typeMap := range dateMap {
		matchesByDate := model.MatchesByDate{
			Date:  date,
			Types: []model.MatchesByType{},
		}

		for sportType, matches := range typeMap {
			matchesByType := model.MatchesByType{
				SportType: sportType,
				Matches:   matches,
			}
			matchesByDate.Types = append(matchesByDate.Types, matchesByType)
		}

		response = append(response, matchesByDate)
	}

	return response
}
