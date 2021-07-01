package transformers

import (
	"encoding/json"
	"scheduler0/server/managers/project"
	"time"
)

// Project paginated container of job transformer
type Project struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ID          int64     `json:"id"`
	UUID        string    `json:"uuid"`
	DateCreated time.Time `json:"date_created"`
}

// PaginatedProject paginated container of project transformer
type PaginatedProject struct {
	Total  int       `json:"total"`
	Offset int       `json:"offset"`
	Limit  int       `json:"limit"`
	Data   []Project `json:"projects"`
}

// ToJSON returns content of transformer as JSON
func (projectTransformer *Project) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(projectTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (projectTransformer *Project) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &projectTransformer); err != nil {
		return err
	}
	return nil
}

// ToManager returns content of transformer as manager
func (projectTransformer *Project) ToManager() project.ProjectManager {
	pd := project.ProjectManager{
		ID:          projectTransformer.ID,
		UUID:        projectTransformer.UUID,
		Name:        projectTransformer.Name,
		Description: projectTransformer.Description,
	}

	return pd
}

// FromManager extracts content from manager into transformer
func (projectTransformer *Project) FromManager(projectManager project.ProjectManager) {
	projectTransformer.ID = projectManager.ID
	projectTransformer.UUID = projectManager.UUID
	projectTransformer.Name = projectManager.Name
	projectTransformer.Description = projectManager.Description
	projectTransformer.DateCreated = projectManager.DateCreated
}
