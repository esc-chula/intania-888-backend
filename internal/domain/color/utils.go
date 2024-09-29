package color

import "github.com/esc-chula/intania-888-backend/internal/model"

func ConvertColorToDto(color *model.Color) *model.ColorDto {
	return &model.ColorDto{
		Id:         color.Id,
		Title:      color.Title,
		Won:        int64(len(color.Won)),
		TotalMatch: int64(color.TotalMatches),
	}
}

func ConvertColorsToDtos(colors []*model.Color) []*model.ColorDto {
	colorDtos := make([]*model.ColorDto, len(colors))
	for i, color := range colors {
		colorDtos[i] = ConvertColorToDto(color)
	}
	return colorDtos
}
