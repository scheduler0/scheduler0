package service

import (
	"errors"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
)

type ProjectService Service

func (projectService *ProjectService) CreateOne(project transformers.Project) (string, error) {
	projectManager := project.ToManager()
	return projectManager.CreateOne(projectService.Pool)
}

func (projectService *ProjectService) UpdateOne(project transformers.Project) error {
	projectManager := project.ToManager()

	count, err := projectManager.UpdateOne(projectService.Pool)
	if err != nil {
		return  err
	}

	if count < 1 {
		return errors.New("cannot find project")
	}

	return nil
}

func (projectService *ProjectService) GetOne(project transformers.Project) (*transformers.Project, error) {
	projectManager := project.ToManager()
	count, err := projectManager.GetOne(projectService.Pool)
	if err != nil {
		return nil, err
	}

	if count < 1 {
		return nil, errors.New("cannot find project")
	}

	project.FromManager(projectManager)

	return &project, nil
}

func (projectService *ProjectService) DeleteOne(project transformers.Project) error {
	projectManager := project.ToManager()
	count, err := projectManager.DeleteOne(projectService.Pool)
	if err != nil {
		return err
	}

	if count < 0 {
		return errors.New("cannot find project")
	}

	return nil
}

func (projectService *ProjectService) List(offset int, limit int) ([]transformers.Project, error) {
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

