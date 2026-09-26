package controllers_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"scheduler0/pkg/http/server/controllers"
	"scheduler0/pkg/mocks"
	"scheduler0/pkg/models"
	"scheduler0/pkg/utils"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupProjectController(t *testing.T) (controllers.ProjectHTTPController, *mocks.MockProjectService) {
	logger := log.New(&bytes.Buffer{}, "", 0)
	mockService := mocks.NewMockProjectService(t)
	controller := controllers.NewProjectController(logger, mockService)
	return controller, mockService
}

func TestProjectController_CreateOneProject(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		project := models.Project{
			Name:      "test-project",
			CreatedBy: "test-user",
		}

		createdProject := &models.Project{
			ID:        100,
			Name:      project.Name,
			CreatedBy: project.CreatedBy,
			AccountId: accountID,
		}

		mockService.On("CreateOne", mock.MatchedBy(func(p models.Project) bool {
			return p.Name == project.Name && p.CreatedBy == project.CreatedBy && p.AccountId == accountID
		})).Return(createdProject, nil)

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneProject(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing createdBy", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		project := models.Project{
			Name: "test-project",
		}

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateOne")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "CreateOne")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		project := models.Project{
			Name:      "test-project",
			CreatedBy: "test-user",
		}

		genericError := &utils.GenericError{
			Message: "failed to create project",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("CreateOne", mock.Anything).Return(nil, genericError)

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.CreateOneProject(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)

		project := models.Project{
			Name:      "test-project",
			CreatedBy: "test-user",
		}

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.CreateOneProject(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "CreateOne")
	})
}

func TestProjectController_GetOneProject(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		projectID := uint64(100)

		mockService.On("GetOneByID", mock.MatchedBy(func(p *models.Project) bool {
			return p.ID == projectID && p.AccountId == accountID
		})).Return(nil).Run(func(args mock.Arguments) {
			p := args.Get(0).(*models.Project)
			p.Name = "test-project"
		})

		req := httptest.NewRequest(http.MethodGet, "/projects/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneProject(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid project ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/projects/invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.GetOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "GetOneByID")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		genericError := &utils.GenericError{
			Message: "project not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("GetOneByID", mock.Anything).Return(genericError)

		req := httptest.NewRequest(http.MethodGet, "/projects/100", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneProject(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)

		req := httptest.NewRequest(http.MethodGet, "/projects/100", nil)
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.GetOneProject(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "GetOneByID")
	})
}

func TestProjectController_ListProjects(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		projects := &models.PaginatedProject{
			Total:  2,
			Offset: 0,
			Limit:  10,
			Data: []models.Project{
				{ID: 1, AccountId: accountID, Name: "project1"},
				{ID: 2, AccountId: accountID, Name: "project2"},
			},
		}

		mockService.On("List", uint64(0), uint64(10), accountID, "date_created", "DESC").Return(projects, nil)

		req := httptest.NewRequest(http.MethodGet, "/projects?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListProjects(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid offset", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/projects?limit=10&offset=invalid", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListProjects(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "List")
	})

	t.Run("invalid limit", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodGet, "/projects?limit=invalid&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListProjects(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "List")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		genericError := &utils.GenericError{
			Message: "failed to list projects",
			Type:    http.StatusInternalServerError,
		}

		mockService.On("List", uint64(0), uint64(10), accountID, "date_created", "DESC").Return(nil, genericError)

		req := httptest.NewRequest(http.MethodGet, "/projects?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithAccountID(accountID))
		w := httptest.NewRecorder()

		controller.ListProjects(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)

		req := httptest.NewRequest(http.MethodGet, "/projects?limit=10&offset=0", nil)
		req = req.WithContext(createContextWithRequestID())
		w := httptest.NewRecorder()

		controller.ListProjects(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "List")
	})
}

func TestProjectController_DeleteOneProject(t *testing.T) {
	t.Run("successful deletion", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		projectID := uint64(100)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		mockService.On("DeleteOneByID", mock.MatchedBy(func(p models.Project) bool {
			return p.ID == projectID && p.AccountId == accountID && p.DeletedBy != nil && *p.DeletedBy == deletedBy
		})).Return(nil)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneProject(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing deletedBy", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		deleteRequest := map[string]string{}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})

	t.Run("invalid project ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/projects/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.DeleteOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		genericError := &utils.GenericError{
			Message: "project not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("DeleteOneByID", mock.Anything).Return(genericError)

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneProject(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		deletedBy := "test-user"

		deleteRequest := map[string]string{
			"deletedBy": deletedBy,
		}

		body, _ := json.Marshal(deleteRequest)
		req := httptest.NewRequest(http.MethodDelete, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.DeleteOneProject(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "DeleteOneByID")
	})
}

func TestProjectController_UpdateOneProject(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		projectID := uint64(100)
		modifiedBy := "test-user"

		project := models.Project{
			ID:         projectID,
			Name:       "updated-project",
			ModifiedBy: &modifiedBy,
		}

		mockService.On("UpdateOneByID", mock.MatchedBy(func(p *models.Project) bool {
			return p.ID == projectID && p.AccountId == accountID && p.ModifiedBy != nil && *p.ModifiedBy == modifiedBy
		})).Return(nil).Run(func(args mock.Arguments) {
			p := args.Get(0).(*models.Project)
			p.Name = "updated-project"
		})

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPut, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing modifiedBy", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		project := models.Project{
			ID:   100,
			Name: "updated-project",
		}

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPut, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneByID")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)

		req := httptest.NewRequest(http.MethodPut, "/projects/100", bytes.NewBuffer([]byte("invalid json")))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneByID")
	})

	t.Run("invalid project ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		modifiedBy := "test-user"

		project := models.Project{
			Name:       "updated-project",
			ModifiedBy: &modifiedBy,
		}

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPut, "/projects/invalid", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneByID")
	})

	t.Run("service error", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		accountID := uint64(1)
		projectID := uint64(100)
		modifiedBy := "test-user"

		project := models.Project{
			ID:         projectID,
			Name:       "updated-project",
			ModifiedBy: &modifiedBy,
		}

		genericError := &utils.GenericError{
			Message: "project not found",
			Type:    http.StatusNotFound,
		}

		mockService.On("UpdateOneByID", mock.Anything).Return(genericError)

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPut, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithAccountID(accountID))
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("missing account ID", func(t *testing.T) {
		controller, mockService := setupProjectController(t)
		projectID := uint64(100)
		modifiedBy := "test-user"

		project := models.Project{
			ID:         projectID,
			Name:       "updated-project",
			ModifiedBy: &modifiedBy,
		}

		body, _ := project.ToJSON()
		req := httptest.NewRequest(http.MethodPut, "/projects/100", bytes.NewBuffer(body))
		req = req.WithContext(createContextWithRequestID())
		req = mux.SetURLVars(req, map[string]string{"id": "100"})
		w := httptest.NewRecorder()

		controller.UpdateOneProject(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockService.AssertNotCalled(t, "UpdateOneByID")
	})
}
