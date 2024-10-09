package color

import (
	"github.com/esc-chula/intania-888-backend/internal/model"
	"go.uber.org/zap"
)

type colorService struct {
	colorRepo ColorRepository
	log       *zap.Logger
}

func NewColorService(colorRepo ColorRepository, log *zap.Logger) ColorService {
	return &colorService{
		colorRepo: colorRepo,
		log:       log,
	}
}

func (s *colorService) GetAllLeaderboards(typeId string) ([]*model.ColorDto, error) {
	colors, err := s.colorRepo.GetAllLeaderboards(typeId)
	if err != nil {
		s.log.Named("GetAllLeaderboards").Error("GetAllColors", zap.Error(err))
		return nil, err
	}

	colorDtos := ConvertColorsToDtos(colors)
	s.log.Named("GetAllLeaderboards").Info("Retrieved all leaderboards successful", zap.Int("count", len(colorDtos)))
	return colorDtos, nil
}

func (s *colorService) GetGroupStageTable(typeId, groupId string) ([]*model.ColorDto, error) {
	colors, err := s.colorRepo.GetGroupStageTable(typeId, groupId)
	if err != nil {
		s.log.Named("GetGroupStageTable").Error("GetAllColors", zap.Error(err))
		return nil, err
	}

	colorDtos := ConvertColorsToDtos(colors)
	s.log.Named("GetGroupStageTable").Info("Retrieved group stage successful", zap.Int("count", len(colorDtos)))
	return colorDtos, nil
}
