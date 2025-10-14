package sporttype

import "github.com/esc-chula/intania-888-backend/internal/model"

func ConvertSportTypeToDto(sportType *model.SportType) *model.SportTypeDto {
	return &model.SportTypeDto{
		Id:    sportType.Id,
		Title: sportType.Title,
	}
}

func ConvertSportTypesToDtos(sportTypes []*model.SportType) []*model.SportTypeDto {
	sportTypeDtos := make([]*model.SportTypeDto, len(sportTypes))
	for i, sportType := range sportTypes {
		sportTypeDtos[i] = ConvertSportTypeToDto(sportType)
	}
	return sportTypeDtos
}
