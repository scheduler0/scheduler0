package controllers_test

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/http/server/controllers"
	async_task "scheduler0-private/pkg/service/async_task"
	"scheduler0-private/pkg/service/node"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupClusterController(t *testing.T) (*controllers.ClusterController, *node.MockNodeService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := node.NewMockNodeService(t)
	mockAsyncTaskService := async_task.NewMockAsyncTaskService(t)
	controller := controllers.NewClusterController(logger, mockService, mockAsyncTaskService)
	return &controller, mockService
}


func TestClusterController_RemoveSelf(t *testing.T) {
	t.Run("successful removal", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		mockService.On("RemoveSelfFromCluster", mock.Anything).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/cluster/remove-self", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).RemoveSelf(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		mockService.On("RemoveSelfFromCluster", mock.Anything).Return(errors.New("removal failed"))

		req := httptest.NewRequest(http.MethodPost, "/cluster/remove-self", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).RemoveSelf(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestClusterController_AddSelf(t *testing.T) {
	t.Run("successful addition", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		mockService.On("AddSelfToCluster", mock.Anything).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/cluster/add-self", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).AddSelf(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		mockService.On("AddSelfToCluster", mock.Anything).Return(errors.New("addition failed"))

		req := httptest.NewRequest(http.MethodPost, "/cluster/add-self", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).AddSelf(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestClusterController_ForceRebuild(t *testing.T) {
	t.Run("successful rebuild", func(t *testing.T) {
		controller, mockService := setupClusterController(t)
		seedNodeID := uint64(1)

		mockService.On("ForceRebuildCluster", mock.Anything, seedNodeID).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/cluster/force-rebuild?seedNodeId=1", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ForceRebuild(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing seedNodeId parameter", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		req := httptest.NewRequest(http.MethodPost, "/cluster/force-rebuild", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ForceRebuild(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ForceRebuildCluster")
	})

	t.Run("invalid seedNodeId parameter", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		req := httptest.NewRequest(http.MethodPost, "/cluster/force-rebuild?seedNodeId=invalid", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ForceRebuild(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "ForceRebuildCluster")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupClusterController(t)
		seedNodeID := uint64(1)

		mockService.On("ForceRebuildCluster", mock.Anything, seedNodeID).Return(errors.New("rebuild failed"))

		req := httptest.NewRequest(http.MethodPost, "/cluster/force-rebuild?seedNodeId=1", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ForceRebuild(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestClusterController_ResetRaft(t *testing.T) {
	t.Run("successful reset", func(t *testing.T) {
		controller, mockService := setupClusterController(t)

		// ResetRaftState exits the process, so we can't fully test it
		// But we can test that the response is sent before the process exits
		mockService.On("ResetRaftState", mock.Anything).Return(nil).Maybe()

		req := httptest.NewRequest(http.MethodPost, "/cluster/reset-raft", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).ResetRaft(w, req)

		// The response should be sent before process exit
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

