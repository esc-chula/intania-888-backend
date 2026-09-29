package match

import (
	"sort"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/domain/location"
)

// ResponseFromSnapshot maps a match embedded in another HTTP resource.
// It leaves the current rates at zero, matching existing bill responses.
func ResponseFromSnapshot(item Snapshot) Response {
	return *matchResultDTO(resultFromSnapshot(item))
}

func matchResultDTO(result *Result) *Response {
	if result == nil {
		return nil
	}

	return &Response{
		ID:         result.ID,
		TeamAID:    result.TeamAID,
		TeamBID:    result.TeamBID,
		TeamAScore: result.TeamAScore,
		TeamBScore: result.TeamBScore,
		TeamARate:  result.TeamARate,
		TeamBRate:  result.TeamBRate,
		WinnerID:   result.WinnerID,
		TypeID:     result.TypeID,
		Location:   location.Response{ID: result.LocationID, Title: result.LocationTitle},
		IsDraw:     result.IsDraw,
		StartTime:  result.StartTime,
		EndTime:    result.EndTime,
	}
}

func matchResultsDTO(results []*Result) []*Response {
	output := make([]*Response, len(results))
	for i := range results {
		output[i] = matchResultDTO(results[i])
	}

	return output
}

func groupMatchesByDateAndType(matches []*Response) []MatchesByDate {
	dateMap := make(map[time.Time]map[string][]*Response)

	for _, match := range matches {
		date := match.StartTime.Truncate(24 * time.Hour)
		sportType := match.TypeID

		if _, dateExists := dateMap[date]; !dateExists {
			dateMap[date] = make(map[string][]*Response)
		}

		dateMap[date][sportType] = append(dateMap[date][sportType], match)
	}

	var response []MatchesByDate

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
		matchesByDate := MatchesByDate{
			Date:  date,
			Types: []MatchesByType{},
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
			matchesByType := MatchesByType{
				SportType: sportType,
				Matches:   matches,
			}

			matchesByDate.Types = append(matchesByDate.Types, matchesByType)
		}

		response = append(response, matchesByDate)
	}

	return response
}
