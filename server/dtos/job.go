package dtos

import (
	"cron-server/server/domains"
	"encoding/json"
	"errors"
	"time"
)

type JobDto struct {
	ID          string `json:"id,omitempty"`
	ProjectId   string `json:"project_id"`
	Description string `json:"description"`
	CronSpec    string `json:"cron_spec,omitempty"`
	Data        string `json:"data,omitempty"`
	Timezone    string `json:"timezone, omitempty"`
	CallbackUrl string `json:"callback_url"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
}

func (IJ *JobDto) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &IJ); err != nil {
		return err
	}
	return nil
}

func (IJ *JobDto) ToJson() ([]byte, error) {
	if data, err := json.Marshal(IJ); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (IJ *JobDto) ToDomain() (domains.JobDomain, error) {
	jd := domains.JobDomain{
		ID:          IJ.ID,
		ProjectId:   IJ.ProjectId,
		CronSpec:    IJ.CronSpec,
		Data:        IJ.Data,
		Description: IJ.Description,
		CallbackUrl: IJ.CallbackUrl,
		Timezone:    IJ.Timezone,
	}

	if len(IJ.StartDate) < 1 {
		return domains.JobDomain{}, errors.New("start date is required")
	}

	startTime, err := time.Parse(time.RFC3339, IJ.StartDate)
	if err != nil {
		return domains.JobDomain{}, err
	}
	jd.StartDate = startTime

	if len(IJ.EndDate) > 1 {
		endTime, err := time.Parse(time.RFC3339, IJ.EndDate)
		if err != nil {
			return domains.JobDomain{}, err
		}

		jd.EndDate = endTime
	}

	return jd, nil
}

func (IJ *JobDto) FromDomain(jd domains.JobDomain) {
	IJ.ID = jd.ID
	IJ.ProjectId = jd.ProjectId
	IJ.Timezone = jd.Timezone
	IJ.Description = jd.Description
	IJ.Data = jd.Data
	IJ.CallbackUrl = jd.CallbackUrl
	IJ.CronSpec = jd.CronSpec
	IJ.StartDate = jd.StartDate.String()
	IJ.EndDate = jd.EndDate.String()
}
