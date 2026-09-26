package feature

import (
	"context"
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"

	sq "github.com/Masterminds/squirrel"
)

type FeatureRepository interface {
	GetFeatures() (*[]models.Feature, *utils.GenericError)
	GetFeatureByID(id uint64) (*models.Feature, *utils.GenericError)
}

type featureRepository struct {
	context  context.Context
	fsmStore fsm.Scheduler0RaftStore
}

func NewFeatureRepository(context context.Context, fsmStore fsm.Scheduler0RaftStore) FeatureRepository {
	return &featureRepository{
		context:  context,
		fsmStore: fsmStore,
	}
}

func (repo *featureRepository) GetFeatures() (*[]models.Feature, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.FeaturesIdColumn,
		constants.FeaturesNameColumn,
		constants.FeaturesDateCreatedColumn,
		constants.FeaturesDateModifiedColumn,
	).
		From(constants.FeaturesTableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()
	var features []models.Feature
	for rows.Next() {
		var feature models.Feature
		scanErr := rows.Scan(
			&feature.ID,
			&feature.Name,
			&feature.CreatedAt,
			&feature.UpdatedAt,
		)
		if scanErr != nil {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
		}
		features = append(features, feature)
	}
	if rows.Err() != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, rows.Err().Error())
	}
	return &features, nil
}

func (repo *featureRepository) GetFeatureByID(id uint64) (*models.Feature, *utils.GenericError) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	selectBuilder := sq.Select(
		constants.FeaturesIdColumn,
		constants.FeaturesNameColumn,
		constants.FeaturesDateCreatedColumn,
		constants.FeaturesDateModifiedColumn,
	).
		From(constants.FeaturesTableName).
		Where(fmt.Sprintf("%s = ?", constants.FeaturesIdColumn), id).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("feature not found with id: %d", id))
	}

	var feature models.Feature
	scanErr := rows.Scan(
		&feature.ID,
		&feature.Name,
		&feature.CreatedAt,
		&feature.UpdatedAt,
	)
	if scanErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, scanErr.Error())
	}

	return &feature, nil
}
