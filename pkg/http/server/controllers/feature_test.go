package controllers_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/http/server/controllers"
	"scheduler0-private/pkg/models"
	feature "scheduler0-private/pkg/service/feature"
	"scheduler0-private/pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupFeatureController(t *testing.T) (*controllers.FeatureController, *feature.MockFeatureService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := feature.NewMockFeatureService(t)
	controller := controllers.NewFeatureController(logger, mockService)
	return controller, mockService
}

func TestFeatureController_GetFeatures(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		controller, mockService := setupFeatureController(t)

		features := &[]models.Feature{
			{
				ID:        1,
				Name:      "feature-1",
				CreatedAt: time.Now(),
			},
			{
				ID:        2,
				Name:      "feature-2",
				CreatedAt: time.Now(),
			},
		}

		mockService.On("GetFeatures").Return(features, (*utils.GenericError)(nil))

		req := httptest.NewRequest(http.MethodGet, "/features", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.GetFeatures(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupFeatureController(t)

		genericError := &utils.GenericError{
			Message: "failed to get features",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("GetFeatures").Return((*[]models.Feature)(nil), genericError)

		req := httptest.NewRequest(http.MethodGet, "/features", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.GetFeatures(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}
