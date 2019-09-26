package models

import "testing"

func TestJob_CreateOne(t *testing.T) {
	// TODO: Test creating a job only accepts only required inbound fields
	// TODO: Test that creating returns error if required inbound fields is missing

	t.Log("")
	{
		var job = InboundJob{}
		job.CallbackUrl = "http://test-url"
		job.Data = "some-data"
		// TODO: Project ID has to be existing
		job.ProjectId = ""
		job.CronSpec  = "* * * * *"

		_, err := job.ToModel().CreateOne()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestJob_GetOne(t *testing.T) {
	// TODO: Getting single job
}

func TestJob_GetAll(t *testing.T) {
	// TODO: Test getting jobs based on project_id, and other custom filters e.g missed_execs
}

func TestJob_UpdateOne(t *testing.T) {
	// TODO: Test updating jobs based on job_id works
}

func TestJob_DeleteOne(t *testing.T) {
	// TODO: Test that deleting a job works
}

