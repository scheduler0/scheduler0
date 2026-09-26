package models

import (
	"encoding/json"
	"time"
)

type ExecutorType string

const (
	ExecutorTypeWebhookUrl    ExecutorType = "webhook_url"
	ExecutorTypeCloudFunction ExecutorType = "cloud_function"
	ExecutorTypeLocal         ExecutorType = "local"
)

type JobExecutor struct {
	ID   uint64 `json:"id" fake:"{number:1,100}"`
	Name string `json:"name" fake:"{number:1,100}"`
	// Description is a free-text explanation of what this executor does. It is used
	// (together with Tags) by the /api/v1/ai/schedule endpoint to let the model pick the
	// executor that best matches a prompt's purpose and channels.
	Description string `json:"description,omitempty" fake:"{sentence:5}"`
	// Tags are short labels describing the executor's purpose/channels (e.g. "email",
	// "sales", "slack"). Persisted as a JSON-encoded TEXT column.
	Tags             []string   `json:"tags,omitempty" fake:"skip"`
	Type             string     `json:"type" fake:"{enum:[cloud_function, webhook_url]}"`
	CloudProvider    string     `json:"cloudProvider" fake:"{string:random}"`
	Region           string     `json:"region" fake:"{string:random}"`
	CloudResourceUrl string     `json:"cloudResourceUrl" fake:"{string:random}"`
	DateCreated      time.Time  `json:"dateCreated" fake:"{date}"`
	AccountId        uint64     `json:"accountId" fake:"{number:1,100}"`
	DateModified     *time.Time `json:"dateModified" fake:"{date}"`
	CreatedBy        string     `json:"createdBy" fake:"{word}"`
	ModifiedBy       *string    `json:"modifiedBy" fake:"{word}"`
	DeletedBy        *string    `json:"deletedBy" fake:"{word}"`
	// CloudApiKey, CloudApiSecret and WebhookSecret are secrets: they are encrypted at
	// rest and MUST NOT be serialized on read responses. They use json:"-" so the default
	// marshaler never leaks them (GET/LIST/UPDATE). They are still accepted as input via
	// the custom UnmarshalJSON below, and revealed exactly once in the create response by
	// the controller (mirroring the credential plaintextSecret pattern).
	CloudApiKey    string `json:"-" fake:"{string:random}"`
	CloudApiSecret string `json:"-" fake:"{string:random}"`
	WebhookUrl     string `json:"webhookUrl" fake:"{string:random}"`
	WebhookSecret  string `json:"-" fake:"{string:random}"`
	WebhookMethod  string `json:"webhookMethod" fake:"{enum:[GET, POST, PUT, DELETE]}"`
	// Command and WorkingDir are used by local executors (type=local). The CLI polls
	// its assigned jobs and runs Command locally in WorkingDir for each trigger.
	Command    string `json:"command" fake:"{word}"`
	WorkingDir string `json:"workingDir" fake:"{word}"`
	// PayloadAggregation, when true, causes jobs sharing this executor and the same
	// scheduled fire time to be delivered in a single aggregated call (see
	// AggregatedJobInvocationPayload) instead of one call per job.
	PayloadAggregation bool `json:"payloadAggregation" fake:"{bool}"`
}

// UnmarshalJSON accepts the secret fields (cloudApiKey, cloudApiSecret, webhookSecret) as
// request input even though they are json:"-" on the struct (which would otherwise make the
// standard decoder ignore them). All other fields decode via their struct tags. The struct's
// default marshaler still omits the secrets, so read responses never expose them.
func (j *JobExecutor) UnmarshalJSON(data []byte) error {
	type alias JobExecutor
	aux := &struct {
		CloudApiKey    *string `json:"cloudApiKey"`
		CloudApiSecret *string `json:"cloudApiSecret"`
		WebhookSecret  *string `json:"webhookSecret"`
		*alias
	}{alias: (*alias)(j)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.CloudApiKey != nil {
		j.CloudApiKey = *aux.CloudApiKey
	}
	if aux.CloudApiSecret != nil {
		j.CloudApiSecret = *aux.CloudApiSecret
	}
	if aux.WebhookSecret != nil {
		j.WebhookSecret = *aux.WebhookSecret
	}
	return nil
}

// PaginatedJobExecutor paginated container of executors transformer
type PaginatedJobExecutor struct {
	Total  uint64        `json:"total,omitempty"`
	Offset uint64        `json:"offset,omitempty"`
	Limit  uint64        `json:"limit,omitempty"`
	Data   []JobExecutor `json:"executors,omitempty"`
}
