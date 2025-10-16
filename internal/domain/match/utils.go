package match

import (
	"sort"
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
		IsDraw:    matchDto.IsDraw,
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
		IsDraw:    match.IsDraw,
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

	// Get all dates and sort them
	var dates []time.Time
	for date := range dateMap {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	// Process dates in sorted order
	for _, date := range dates {
		typeMap := dateMap[date]
		matchesByDate := model.MatchesByDate{
			Date:  date,
			Types: []model.MatchesByType{},
		}

		// Get all sport types and sort them alphabetically for consistency
		var sportTypes []string
		for sportType := range typeMap {
			sportTypes = append(sportTypes, sportType)
		}
		sort.Strings(sportTypes)

		// Process sport types in sorted order
		for _, sportType := range sportTypes {
			matches := typeMap[sportType]
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

func calculateOddsRate(betOn string, totalBetOnA, totalBetOnB float64) float64 {
	total := totalBetOnA + totalBetOnB

	if total == 0 {
		return 0
	}

	switch betOn {
	case "A":
		if totalBetOnA == 0 {
			return 0
		}
		return total / totalBetOnA
	case "B":
		if totalBetOnB == 0 {
			return 0
		}
		return total / totalBetOnB
	default:
		return 0
	}
}

func calculatePayout(totalRates, amount float64) float64 {
	payout := totalRates * amount
	return roundToTwoDecimals(payout)
}

func roundToTwoDecimals(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
