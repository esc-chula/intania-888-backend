package sporttype

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"go.uber.org/zap"
)

type sportTypeService struct {
	sportTypeRepo SportTypeRepository
	log           *zap.Logger
}

func NewSportTypeService(sportTypeRepo SportTypeRepository, log *zap.Logger) SportTypeService {
	return &sportTypeService{
		sportTypeRepo: sportTypeRepo,
		log:           log,
	}
}

func (s *sportTypeService) GetAllSportTypes() ([]*model.SportTypeDto, error) {
	sportTypes, err := s.sportTypeRepo.GetAllSportTypes()
	if err != nil {
		s.log.Named("GetAllSportTypes").Error("Failed to get sport types", zap.Error(err))
		return nil, err
	}

	sportTypeDtos := ConvertSportTypesToDtos(sportTypes)
	s.log.Named("GetAllSportTypes").Info("Retrieved all sport types successful", zap.Int("count", len(sportTypeDtos)))
	return sportTypeDtos, nil
}
