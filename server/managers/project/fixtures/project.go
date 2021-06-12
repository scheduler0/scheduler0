package fixtures

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	projectManager "scheduler0/server/managers/project"
	"scheduler0/server/transformers"
)

// ProjectFixture project test fixtures
type ProjectFixture struct {
	Name        string `faker:"username"`
	Description string `faker:"username"`
}

// CreateProjectTransformerFixture creates a new project transformer
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

// CreateProjectManagerFixture creates a new project test fixture
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
