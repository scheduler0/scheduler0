package service

import (
	"fmt"
	"net/http"
	"scheduler0/server/transformers"
	"scheduler0/utils"
)

// ProjectService project server the layer on top db managers
type ProjectService Service


// CreateOne creates a new project
func (projectService *ProjectService) CreateOne(project transformers.Project) (*transformers.Project, *utils.GenericError) {
	projectManager := project.ToManager()

	_, err := projectManager.CreateOne(projectService.Pool)
	if err != nil {
		return nil, err
	}

	projectTransformer := transformers.Project{}

	projectTransformer.FromManager(projectManager)

	return &projectTransformer, nil
}

// UpdateOne updates a single project
func (projectService *ProjectService) UpdateOne(project transformers.Project) *utils.GenericError {
	projectManager := project.ToManager()

	count, err := projectManager.UpdateOne(projectService.Pool)
	if err != nil {
		return err
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", projectManager.UUID))
	}

	return nil
}

// GetOneByUUID returns project with matching uuid
func (projectService *ProjectService) GetOneByUUID(project transformers.Project) (*transformers.Project, *utils.GenericError) {
	projectManager := project.ToManager()
	err := projectManager.GetOneByUUID(projectService.Pool)
	if err != nil {
		return nil, err
	}

	project.FromManager(projectManager)

	return &project, nil
}


// GetOneByName returns a project that matches the name
func (projectService *ProjectService) GetOneByName(project transformers.Project) (*transformers.Project, *utils.GenericError) {
	projectManager := project.ToManager()
	err := projectManager.GetOneByName(projectService.Pool)
	if err != nil {
		return nil, err
	}

	project.FromManager(projectManager)

	return &project, nil
}

// DeleteOne deletes a single project
func (projectService *ProjectService) DeleteOne(project transformers.Project) *utils.GenericError {
	projectManager := project.ToManager()
	count, err := projectManager.DeleteOne(projectService.Pool)
	if err != nil {
		return err
	}

	if count < 0 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", projectManager.UUID))
	}

	return nil
}


// List return a paginated list of projects
func (projectService *ProjectService) List(offset int, limit int) (*transformers.PaginatedProject, *utils.GenericError) {
	project := transformers.Project{}
	projectManager := project.ToManager()
	projects, err := projectManager.GetAll(projectService.Pool, offset, limit)
	if err != nil {
		return nil, err
	}

	count, err := projectManager.Count(projectService.Pool)
	if err != nil {
		return nil, err
	}

	transformedProjects := make([]transformers.Project, 0, len(projects))

	for _, project := range projects {
		transformedProject := transformers.Project{}
		transformedProject.FromManager(project)
		transformedProjects = append(transformedProjects, transformedProject)
	}

	paginatedProjects := transformers.PaginatedProject{}

	paginatedProjects.Total = count
	paginatedProjects.Data = transformedProjects
	paginatedProjects.Limit = limit
	paginatedProjects.Offset = offset

	return &paginatedProjects, nil
}
