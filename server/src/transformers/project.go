package transformers

import (
	"cron-server/server/src/managers"
	"encoding/json"
	"time"
)

type Project struct {
	Name        string    `json:"name" pg:",notnull"`
	Description string    `json:"description" pg:",notnull"`
	ID          string    `json:"id" pg:",notnull"`
	DateCreated time.Time `json:"date_created" pg:",notnull"`
}

func (p *Project) ToJson() ([]byte, error) {
	if data, err := json.Marshal(p); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (p *Project) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	return nil
}

func (p *Project) ToManager() managers.ProjectManager {
	pd := managers.ProjectManager{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}

	return pd
}

func (p *Project) FromManager(pd managers.ProjectManager) {
	p.ID = pd.ID
	p.Name = pd.Name
	p.Description = pd.Description
	p.DateCreated = pd.DateCreated
}
