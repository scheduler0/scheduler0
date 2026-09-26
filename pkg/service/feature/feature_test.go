package feature

import (
	"net/http"
	"scheduler0-private/pkg/mocks"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_FeatureService_GetFeatures_Success(t *testing.T) {
	// Setup mock repository with test data
	now := time.Now()
	testFeatures := []models.Feature{
		{
			ID:        1,
			Name:      "feature-1",
			CreatedAt: now,
			UpdatedAt: &now,
		},
		{
			ID:        2,
			Name:      "feature-2",
			CreatedAt: now,
			UpdatedAt: &now,
		},
	}

	mockRepo := mocks.NewMockFeatureRepository(t)
	mockRepo.EXPECT().GetFeatures().Return(&testFeatures, nil)

	// Create service with mock repository
	service := NewFeatureService(mockRepo)

	// Test GetFeatures
	features, err := service.GetFeatures()

	// Assertions
	assert.Nil(t, err)
	assert.NotNil(t, features)
	assert.Equal(t, 2, len(*features))
	assert.Equal(t, uint64(1), (*features)[0].ID)
	assert.Equal(t, "feature-1", (*features)[0].Name)
	assert.Equal(t, uint64(2), (*features)[1].ID)
	assert.Equal(t, "feature-2", (*features)[1].Name)
}

func Test_FeatureService_GetFeatures_EmptyList(t *testing.T) {
	// Setup mock repository with empty list
	emptyFeatures := []models.Feature{}
	mockRepo := mocks.NewMockFeatureRepository(t)
	mockRepo.EXPECT().GetFeatures().Return(&emptyFeatures, nil)

	// Create service with mock repository
	service := NewFeatureService(mockRepo)

	// Test GetFeatures
	features, err := service.GetFeatures()

	// Assertions
	assert.Nil(t, err)
	assert.NotNil(t, features)
	assert.Equal(t, 0, len(*features))
}

func Test_FeatureService_GetFeatures_RepositoryError(t *testing.T) {
	// Setup mock repository with error
	expectedError := utils.HTTPGenericError(http.StatusInternalServerError, "database connection failed")
	mockRepo := mocks.NewMockFeatureRepository(t)
	mockRepo.EXPECT().GetFeatures().Return(nil, expectedError)

	// Create service with mock repository
	service := NewFeatureService(mockRepo)

	// Test GetFeatures
	features, err := service.GetFeatures()

	// Assertions
	assert.NotNil(t, err)
	assert.Nil(t, features)
	assert.Equal(t, expectedError, err)
	assert.Equal(t, http.StatusInternalServerError, err.Type)
	assert.Equal(t, "database connection failed", err.Message)
}

func Test_FeatureService_NewFeatureService(t *testing.T) {
	// Test service creation
	mockRepo := mocks.NewMockFeatureRepository(t)
	service := NewFeatureService(mockRepo)

	// Assertions
	assert.NotNil(t, service)

	// Verify it implements the interface
	var _ FeatureService = service
}
