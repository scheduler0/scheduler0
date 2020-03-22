package dtos

import (
	"cron-server/server/domains"
	"encoding/json"
	"time"
)

type ProjectDto struct {
	Name        string    `json:"name" pg:",notnull"`
	Description string    `json:"description" pg:",notnull"`
	ID          string    `json:"id" pg:",notnull"`
	DateCreated time.Time `json:"date_created" pg:",notnull"`
}

func (p *ProjectDto) ToJson() ([]byte, error) {
	if data, err := json.Marshal(p); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (p *ProjectDto) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	return nil
}

func (p *ProjectDto) ToDomain() (domains.ProjectDomain, error) {
	pd := domains.ProjectDomain{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}

	return pd, nil
}

func (p *ProjectDto) FromDomain(pd domains.ProjectDomain) {
	p.ID = pd.ID
	p.Name = pd.Name
	p.Description = pd.Description
	p.DateCreated = pd.DateCreated
}
