package service

import (
	"fmt"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"net/http"
)

type ProjectService Service

func (projectService *ProjectService) CreateOne(project transformers.Project) (string, *utils.GenericError) {
	projectManager := project.ToManager()
	return projectManager.CreateOne(projectService.Pool)
}

func (projectService *ProjectService) UpdateOne(project transformers.Project) *utils.GenericError {
	projectManager := project.ToManager()

	count, err := projectManager.UpdateOne(projectService.Pool)
	if err != nil {
		return  err
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("Cannot find ProjectUUID = %v", projectManager.UUID))
	}

	return nil
}

func (projectService *ProjectService) GetOneByUUID(project transformers.Project) (*transformers.Project, *utils.GenericError) {
	projectManager := project.ToManager()
	err := projectManager.GetOneByUUID(projectService.Pool)
	if err != nil {
		return nil, err
	}

	project.FromManager(projectManager)

	return &project, nil
}

func (projectService *ProjectService) GetOneByName(project transformers.Project) (*transformers.Project, *utils.GenericError) {
	projectManager := project.ToManager()
	err := projectManager.GetOneByName(projectService.Pool)
	if err != nil {
		return nil, err
	}

	project.FromManager(projectManager)

	return &project, nil
}

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

func (projectService *ProjectService) List(offset int, limit int) ([]transformers.Project, *utils.GenericError) {
	project := transformers.Project{}
	projectManager := project.ToManager()
	projects, err := projectManager.GetAll(projectService.Pool, offset, limit)
	if err != nil {
		return nil, err
	}

	transformedProjects := make([]transformers.Project, 0, len(projects))

	for _, project := range projects {
		transformedProject := transformers.Project{}
		transformedProject.FromManager(project)
		transformedProjects = append(transformedProjects, transformedProject)
	}

	return transformedProjects, nil
}

