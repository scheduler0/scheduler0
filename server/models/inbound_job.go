package models

import (
	"encoding/json"
	"errors"
	"time"
)

type InboundJob struct {
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

func (IJ *InboundJob) ToModel() (Job, error) {
	jd := Job{
		ID:          IJ.ID,
		ProjectId:   IJ.ProjectId,
		CronSpec:    IJ.CronSpec,
		Data:        IJ.Data,
		Description: IJ.Description,
		CallbackUrl: IJ.CallbackUrl,
		Timezone:    IJ.Timezone,
	}

	if len(IJ.StartDate) < 1 {
		return Job{}, errors.New("start date is required")
	}

	startTime, err := time.Parse(time.RFC1123, IJ.StartDate)
	if err != nil {
		return Job{}, err
	}
	jd.StartDate = startTime

	if len(IJ.EndDate) > 1 {
		endTime, err := time.Parse(time.RFC1123, IJ.EndDate)
		if err != nil {
			return Job{}, err
		}

		jd.EndDate = endTime
	}

	return jd, nil
}

func (IJ *InboundJob) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &IJ); err != nil {
		return err
	}
	return nil
}

func (IJ *InboundJob) ToJson() ([]byte, error) {
	if data, err := json.Marshal(IJ); err != nil {
		return data, err
	} else {
		return data, nil
	}
}
