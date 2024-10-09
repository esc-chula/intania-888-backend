package color

import "github.com/esc-chula/intania-888-backend/internal/model"

type ColorService interface {
	GetAllLeaderboards(typeId string) ([]*model.ColorDto, error)
	GetGroupStageTable(typeId, groupId string) ([]*model.ColorDto, error)
}

type ColorRepository interface {
	GetAllLeaderboards(typeId string) ([]*model.Color, error)
	GetGroupStageTable(typeId, groupId string) ([]*model.Color, error)
}
