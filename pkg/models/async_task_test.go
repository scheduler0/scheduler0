package models

import (
	"bytes"
	"testing"
	"time"
)

func TestAsyncTask_Bytes(t *testing.T) {
	at := &AsyncTask{
		Id:          1,
		RequestId:   "req-123",
		Input:       "input data",
		Output:      "output data",
		Service:     "test-service",
		State:       AsyncTaskSuccess,
		DateCreated: time.Now(),
	}

	bytes := at.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestAsyncTask_String(t *testing.T) {
	at := &AsyncTask{
		Id:          1,
		RequestId:   "req-123",
		Input:       "input data",
		Output:      "output data",
		Service:     "test-service",
		State:       AsyncTaskSuccess,
		DateCreated: time.Now(),
	}

	str := at.String()
	if len(str) == 0 {
		t.Errorf("Expected String() to return non-empty string")
	}
}

func TestAsyncTask_WriteTo(t *testing.T) {
	at := &AsyncTask{
		Id:          1,
		RequestId:   "req-123",
		Input:       "input data",
		Output:      "output data",
		Service:     "test-service",
		State:       AsyncTaskSuccess,
		DateCreated: time.Now(),
	}

	var buf bytes.Buffer
	n, err := at.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}
}

func TestAsyncTask_ReadFrom(t *testing.T) {
	// First write an AsyncTask
	at := &AsyncTask{
		Id:          1,
		RequestId:   "req-123",
		Input:       "input data",
		Output:      "output data",
		Service:     "test-service",
		State:       AsyncTaskSuccess,
		DateCreated: time.Now(),
	}
	var writeBuf bytes.Buffer
	_, _ = at.WriteTo(&writeBuf)

	// Now read it back
	var readAT AsyncTask
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readAT.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readAT.Id != at.Id {
		t.Errorf("Expected ReadFrom to recover Id, got %d, expected %d", readAT.Id, at.Id)
	}

	if readAT.RequestId != at.RequestId {
		t.Errorf("Expected ReadFrom to recover RequestId, got %s, expected %s", readAT.RequestId, at.RequestId)
	}
}

func TestAsyncTask_ReadFrom_InvalidType(t *testing.T) {
	var readAT AsyncTask
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))  // Size

	_, err := readAT.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestAsyncTask_ReadFrom_Empty(t *testing.T) {
	var readAT AsyncTask
	var buf bytes.Buffer

	_, err := readAT.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestAsyncTask_ReadFrom_MaxPayloadSize(t *testing.T) {
	var readAT AsyncTask
	var buf bytes.Buffer

	// Write payload type
	buf.WriteByte(AsyncTaskPayload)
	// Write size that exceeds MaxPayloadSize
	size := MaxPayloadSize + 1
	var sizeBytes [4]byte
	sizeBytes[0] = byte(size >> 24)
	sizeBytes[1] = byte(size >> 16)
	sizeBytes[2] = byte(size >> 8)
	sizeBytes[3] = byte(size)
	buf.Write(sizeBytes[:])

	_, err := readAT.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for payload exceeding MaxPayloadSize, got nil")
	}
	if err != ErrMaxPayloadSize {
		t.Errorf("Expected ReadFrom to return ErrMaxPayloadSize, got %v", err)
	}
}

func TestAsyncTask_RoundTrip(t *testing.T) {
	testCases := []struct {
		id          uint64
		requestId   string
		input       string
		output      string
		service     string
		state       AsyncTaskState
		dateCreated time.Time
	}{
		{1, "req-1", "input1", "output1", "service1", AsyncTaskSuccess, time.Now()},
		{2, "req-2", "", "", "service2", AsyncTaskFail, time.Now().Add(-1 * time.Hour)},
		{3, "req-3", "long input data", "long output data", "test-service", AsyncTaskInProgress, time.Now()},
	}

	for _, tc := range testCases {
		at := &AsyncTask{
			Id:          tc.id,
			RequestId:   tc.requestId,
			Input:       tc.input,
			Output:      tc.output,
			Service:     tc.service,
			State:       tc.state,
			DateCreated: tc.dateCreated,
		}

		var buf bytes.Buffer
		_, err := at.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readAT AsyncTask
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readAT.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if readAT.Id != at.Id {
			t.Errorf("Round trip failed for Id: got %d, expected %d", readAT.Id, at.Id)
		}
		if readAT.RequestId != at.RequestId {
			t.Errorf("Round trip failed for RequestId: got %s, expected %s", readAT.RequestId, at.RequestId)
		}
		if readAT.Service != at.Service {
			t.Errorf("Round trip failed for Service: got %s, expected %s", readAT.Service, at.Service)
		}
		if readAT.State != at.State {
			t.Errorf("Round trip failed for State: got %d, expected %d", readAT.State, at.State)
		}
	}
}

