package project

import (
	"fmt"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/project"
	"scheduler0/pkg/utils"

	"github.com/hashicorp/go-hclog"
)

// JobService interface for job operations
type JobService interface {
	DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) *utils.GenericError
}

// ProjectService project server the layer on top db repos
type projectService struct {
	projectRepo project.ProjectRepo
	jobService  JobService
	logger      hclog.Logger
}

type ProjectService interface {
	CreateOne(project models.Project) (*models.Project, *utils.GenericError)
	UpdateOneByID(project *models.Project) *utils.GenericError
	GetOneByID(project *models.Project) *utils.GenericError
	GetOneByName(project *models.Project) *utils.GenericError
	DeleteOneByID(project models.Project) *utils.GenericError
	List(offset uint64, limit uint64, accountId uint64, orderByColumn string, orderByDirection string) (*models.PaginatedProject, *utils.GenericError)
	BatchGetProjects(projectIds []uint64) ([]models.Project, *utils.GenericError)
}

func NewProjectService(logger hclog.Logger, projectRepo project.ProjectRepo, jobService JobService) ProjectService {
	return &projectService{
		projectRepo: projectRepo,
		jobService:  jobService,
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
	// Validate that deletedBy is provided
	if project.DeletedBy == nil || *project.DeletedBy == "" {
		return utils.HTTPGenericError(http.StatusBadRequest, "deletedBy is required")
	}

	if project.AccountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	// Validate that account ID is not 1 (system user)
	if project.AccountId == 1 {
		return utils.HTTPGenericError(http.StatusBadRequest, "system projects cannot be deleted")
	}

	// Verify the project belongs to the caller before cascading deletes.
	existing := models.Project{ID: project.ID, AccountId: project.AccountId}
	if getErr := projectService.GetOneByID(&existing); getErr != nil {
		return getErr
	}

	// Delete all jobs for this project first (account-scoped)
	deleteJobsErr := projectService.jobService.DeleteJobsByProjectID(project.ID, project.AccountId, *project.DeletedBy)
	if deleteJobsErr != nil {
		projectService.logger.Error("Failed to delete jobs for project", "projectID", project.ID, "accountId", project.AccountId, "error", deleteJobsErr)
		return deleteJobsErr
	}

	// Delete the project
	count, err := projectService.projectRepo.DeleteOneByID(project)
	if err != nil {
		return err
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", project.ID))
	}

	projectService.logger.Info("Successfully deleted project and its jobs", "projectID", project.ID, "accountId", project.AccountId, "deletedBy", *project.DeletedBy)
	return nil
}

// List return a paginated list of projects
func (projectService *projectService) List(offset uint64, limit uint64, accountId uint64, orderByColumn string, orderByDirection string) (*models.PaginatedProject, *utils.GenericError) {
	if limit > constants.MaxListLimit {
		return nil, utils.HTTPGenericError(http.StatusTooManyRequests, fmt.Sprintf("too many projects. limit should be less than %d", constants.MaxListLimit))
	}

	if limit < 1 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "limit should be greater than 0")
	}

	if offset < 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "offset should be greater than 0")
	}

	projects, err := projectService.projectRepo.List(offset, limit, accountId, orderByColumn, orderByDirection)
	if err != nil {
		return nil, err
	}

	count, err := projectService.projectRepo.Count(accountId)
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
