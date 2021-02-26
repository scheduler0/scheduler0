package fixtures

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	projectManager "scheduler0/server/managers/project"
	"scheduler0/server/transformers"
)

type ProjectFixture struct {
	Name        string `faker:"username"`
	Description string `faker:"username"`
}

func CreateProjectTransformerFixture() transformers.Project {
	project := ProjectFixture{}
	err := faker.FakeData(&project)
	if err != nil {
		fmt.Printf("[ERROR] failed to project fixture")
	}
	projectTransformer := transformers.Project{
		Name:        project.Name,
		Description: project.Description,
	}
	return projectTransformer
}

func CreateProjectManagerFixture() projectManager.ProjectManager {
	project := ProjectFixture{}
	err := faker.FakeData(&project)
	if err != nil {
		fmt.Printf("[ERROR] failed to project fixture")
	}

	return projectManager.ProjectManager{
		Name:        project.Name,
		Description: project.Description,
	}
}
