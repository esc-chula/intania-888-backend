package sporttype

import "github.com/esc-chula/intania-888-backend/internal/model"

type SportTypeService interface {
	GetAllSportTypes() ([]*model.SportTypeDto, error)
}

type SportTypeRepository interface {
	GetAllSportTypes() ([]*model.SportType, error)
}
