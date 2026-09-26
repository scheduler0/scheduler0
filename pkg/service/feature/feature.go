package feature

import (
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/feature"
	"scheduler0/pkg/utils"
)

type FeatureService interface {
	GetFeatures() (*[]models.Feature, *utils.GenericError)
}

type featureService struct {
	featureRepository feature.FeatureRepository
}

func NewFeatureService(featureRepository feature.FeatureRepository) FeatureService {
	return &featureService{
		featureRepository: featureRepository,
	}
}

func (service *featureService) GetFeatures() (*[]models.Feature, *utils.GenericError) {
	return service.featureRepository.GetFeatures()
}
