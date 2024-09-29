package color

import "github.com/esc-chula/intania-888-backend/internal/model"

type ColorService interface {
	GetAllLeaderboards(typeId string) ([]*model.ColorDto, error)
}

type ColorRepository interface {
	GetAllColors(typeId string) ([]*model.Color, error)
}
