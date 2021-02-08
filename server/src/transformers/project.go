package transformers

import (
	"encoding/json"
	"github.com/victorlenerd/scheduler0/server/src/managers/project"
	"time"
)

type Project struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UUID        string    `json:"uuid"`
	DateCreated time.Time `json:"date_created"`
}

func (projectTransformer *Project) ToJson() ([]byte, error) {
	if data, err := json.Marshal(projectTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (projectTransformer *Project) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &projectTransformer); err != nil {
		return err
	}
	return nil
}

func (projectTransformer *Project) ToManager() project.ProjectManager {
	pd := project.ProjectManager{
		UUID:        projectTransformer.UUID,
		Name:        projectTransformer.Name,
		Description: projectTransformer.Description,
	}

	return pd
}

func (projectTransformer *Project) FromManager(projectManager project.ProjectManager) {
	projectTransformer.UUID = projectManager.UUID
	projectTransformer.Name = projectManager.Name
	projectTransformer.Description = projectManager.Description
	projectTransformer.DateCreated = projectManager.DateCreated
}
