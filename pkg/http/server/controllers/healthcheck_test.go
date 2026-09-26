package controllers_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/service/node"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func setupHealthCheckController(t *testing.T) (*controllers.HealthCheckController, *node.MockNodeService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := node.NewMockNodeService(t)
	controller := controllers.NewHealthCheckController(logger, mockService)
	return &controller, mockService
}


func TestHealthCheckController_HealthCheck(t *testing.T) {
	t.Run("successful health check", func(t *testing.T) {
		controller, mockService := setupHealthCheckController(t)

		leaderAddress := raft.ServerAddress("127.0.0.1:8080")
		leaderID := raft.ServerID("leader-1")
		raftStats := map[string]string{
			"state": "Leader",
		}

		mockService.On("GetRaftLeaderWithId").Return(leaderAddress, leaderID)
		mockService.On("GetRaftStats").Return(raftStats)

		req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		(*controller).HealthCheck(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})
}

