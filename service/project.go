package service

import (
	"fmt"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
)

// ProjectService project server the layer on top db repos
type projectService struct {
	projectRepo repository.Project
}

type Project interface {
	CreateOne(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError)
	UpdateOneByUUID(project models.ProjectModel) *utils.GenericError
	GetOneByUUID(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError)
	GetOneByName(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError)
	DeleteOneByUUID(project models.ProjectModel) *utils.GenericError
	List(offset int64, limit int64) (*models.PaginatedProject, *utils.GenericError)
}

func NewProjectService(projectRepo repository.Project) Project {
	return &projectService{
		projectRepo: projectRepo,
	}
}

// CreateOne creates a new project
func (projectService *projectService) CreateOne(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError) {
	_, err := projectService.projectRepo.CreateOne(project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateOneByUUID updates a single project
func (projectService *projectService) UpdateOneByUUID(project models.ProjectModel) *utils.GenericError {
	count, err := projectService.projectRepo.UpdateOneByID(project)
	if err != nil {
		return err
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", project.ID))
	}

	return nil
}

// GetOneByUUID returns project with matching uuid
func (projectService *projectService) GetOneByUUID(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError) {
	err := projectService.projectRepo.GetOneByID(&project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetOneByName returns a project that matches the name
func (projectService *projectService) GetOneByName(project models.ProjectModel) (*models.ProjectModel, *utils.GenericError) {
	err := projectService.projectRepo.GetOneByName(&project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteOneByUUID deletes a single project
func (projectService *projectService) DeleteOneByUUID(project models.ProjectModel) *utils.GenericError {
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
func (projectService *projectService) List(offset int64, limit int64) (*models.PaginatedProject, *utils.GenericError) {
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
