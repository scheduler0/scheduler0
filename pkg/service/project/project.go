package project

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/project"
	"scheduler0/pkg/utils"
)

// ProjectService project server the layer on top db repos
type projectService struct {
	projectRepo project.ProjectRepo
	logger      hclog.Logger
}

//go:generate mockery --name ProjectService --output ../mocks
type ProjectService interface {
	CreateOne(project models.Project) (*models.Project, *utils.GenericError)
	UpdateOneByID(project *models.Project) *utils.GenericError
	GetOneByID(project *models.Project) *utils.GenericError
	GetOneByName(project *models.Project) *utils.GenericError
	DeleteOneByID(project models.Project) *utils.GenericError
	List(offset uint64, limit uint64) (*models.PaginatedProject, *utils.GenericError)
	BatchGetProjects(projectIds []uint64) ([]models.Project, *utils.GenericError)
}

func NewProjectService(logger hclog.Logger, projectRepo project.ProjectRepo) ProjectService {
	return &projectService{
		projectRepo: projectRepo,
		logger:      logger.Named("project-service"),
	}
}

// CreateOne creates a new project
func (projectService *projectService) CreateOne(project models.Project) (*models.Project, *utils.GenericError) {
	_, err := projectService.projectRepo.CreateOne(&project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (projectService *projectService) UpdateOneByID(project *models.Project) *utils.GenericError {
	if len(project.Name) < 1 {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("project name is required"))
	}

	count, err := projectService.projectRepo.UpdateOneByID(*project)
	if err != nil {
		return err
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectID = %v", project.ID))
	}

	getErr := projectService.GetOneByID(project)
	if getErr != nil {
		return getErr
	}

	return nil
}

func (projectService *projectService) GetOneByID(project *models.Project) *utils.GenericError {
	err := projectService.projectRepo.GetOneByID(project)
	if err != nil {
		return err
	}

	return nil
}

// GetOneByName returns a project that matches the name
func (projectService *projectService) GetOneByName(project *models.Project) *utils.GenericError {
	err := projectService.projectRepo.GetOneByName(project)
	if err != nil {
		return err
	}
	return nil
}

// DeleteOneByID deletes a single project
func (projectService *projectService) DeleteOneByID(project models.Project) *utils.GenericError {
	count, err := projectService.projectRepo.DeleteOneByID(project)
	if err != nil {
		return err
	}

	if count < 0 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", project.ID))
	}

	return nil
}

// List return a paginated list of projects
func (projectService *projectService) List(offset uint64, limit uint64) (*models.PaginatedProject, *utils.GenericError) {
	projects, err := projectService.projectRepo.List(offset, limit)
	if err != nil {
		return nil, err
	}

	count, err := projectService.projectRepo.Count()
	if err != nil {
		return nil, err
	}

	paginatedProjects := models.PaginatedProject{}

	paginatedProjects.Total = count
	paginatedProjects.Data = projects
	paginatedProjects.Limit = limit
	paginatedProjects.Offset = offset

	return &paginatedProjects, nil
}

func (projectService *projectService) BatchGetProjects(projectIds []uint64) ([]models.Project, *utils.GenericError) {
	if len(projectIds) < 1 {
		return []models.Project{}, nil
	}

	projects, err := projectService.projectRepo.GetBatchProjectsByIDs(projectIds)
	if err != nil {
		return nil, err
	}

	return projects, nil
}
