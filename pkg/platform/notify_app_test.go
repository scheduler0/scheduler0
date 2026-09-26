package platform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"scheduler0-private/pkg/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyJobFailure_NoopWhenURLEmpty(t *testing.T) {
	n := NewNotifier("", "secret")
	err := n.NotifyJobFailure(models.Job{ID: 1, AccountId: 2}, 3, 7)
	assert.NoError(t, err)
}

func TestNotifyJobFailure_PostsExpectedPayload(t *testing.T) {
	var gotMethod, gotSecret, gotContentType string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotSecret = r.Header.Get("X-Webhook-Secret")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "top-secret")
	err := n.NotifyJobFailure(models.Job{
		ID:        123,
		AccountId: 2,
		ProjectID: 456,
		Status:    models.JobStatusActive,
	}, 3, 7)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "top-secret", gotSecret)
	assert.Equal(t, "application/json", gotContentType)

	var payload models.JobInvocationPayload
	require.NoError(t, json.Unmarshal(gotBody, &payload))
	assert.Equal(t, "failed", payload.LastExecutionStatus)
	assert.Equal(t, uint64(123), payload.Job.ID)

	var data JobFailureData
	require.NoError(t, json.Unmarshal([]byte(payload.Job.Data), &data))
	assert.Equal(t, JobFailureJobType, data.JobType)
	assert.Equal(t, "2", data.AccountID)
	assert.Equal(t, "123", data.JobID)
	assert.Equal(t, "456", data.ProjectID)
	assert.Equal(t, uint64(3), data.FailCount)
	assert.Equal(t, uint64(7), data.ExecutionVersion)
}

func TestNotifyJobFailure_ReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "secret")
	err := n.NotifyJobFailure(models.Job{ID: 1, AccountId: 2}, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
