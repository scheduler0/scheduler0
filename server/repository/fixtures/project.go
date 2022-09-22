package fixtures

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	"scheduler0/server/models"
)

// ProjectFixture project test fixtures
type ProjectFixture struct {
	Name        string `faker:"username"`
	Description string `faker:"username"`
}

// CreateProjectTransformerFixture creates a new project transformer
func CreateProjectTransformerFixture() models.ProjectModel {
	project := ProjectFixture{}
	err := faker.FakeData(&project)
	if err != nil {
		fmt.Printf("[ERROR] failed to project fixture")
	}
	projectTransformer := models.ProjectModel{
		Name:        project.Name,
		Description: project.Description,
	}
	return projectTransformer
}

// CreateProjectManagerFixture creates a new project test fixture
func CreateProjectManagerFixture() models.ProjectModel {
	project := ProjectFixture{}
	err := faker.FakeData(&project)
	if err != nil {
		fmt.Printf("[ERROR] failed to project fixture")
	}

	return models.ProjectModel{
		Name:        project.Name,
		Description: project.Description,
	}
}
