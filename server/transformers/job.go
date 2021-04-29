package transformers

import (
	"encoding/json"
	"scheduler0/server/managers/job"
)

// Job transformer type
type Job struct {
	UUID        string `json:"uuid,omitempty"`
	ProjectUUID string `json:"project_uuid"`
	Spec        string `json:"spec,omitempty"`
	Data        string `json:"transformers,omitempty"`
	CallbackUrl string `json:"callback_url"`
}

// PaginatedJob paginated container of job transformer
type PaginatedJob struct {
	Total  int   `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Data   []Job `json:"jobs"`
}

// ToJSON returns content of transformer as JSON
func (jobTransformer *Job) ToJSON() ([]byte, error) {
	if data, err := json.Marshal(jobTransformer); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

// FromJSON extracts content of JSON object into transformer
func (jobTransformer *Job) FromJSON(body []byte) error {
	if err := json.Unmarshal(body, &jobTransformer); err != nil {
		return err
	}
	return nil
}

// ToManager returns content of transformer as manager
func (jobTransformer *Job) ToManager() (job.Manager, error) {
	jobManager := job.Manager{
		UUID:        jobTransformer.UUID,
		ProjectUUID: jobTransformer.ProjectUUID,
		Spec:        jobTransformer.Spec,
		CallbackUrl: jobTransformer.CallbackUrl,
	}

	return jobManager, nil
}

// FromManager extracts content from manager into transformer
func (jobTransformer *Job) FromManager(jobManager job.Manager) {
	jobTransformer.UUID = jobManager.UUID
	jobTransformer.ProjectUUID = jobManager.ProjectUUID
	jobTransformer.CallbackUrl = jobManager.CallbackUrl
	jobTransformer.Spec = jobManager.Spec
}
