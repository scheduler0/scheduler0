package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSendJSON(t *testing.T) {
	// Create a new HTTP request recorder
	recorder := httptest.NewRecorder()

	// Define the response data
	data := map[string]interface{}{
		"message": "Hello, world!",
	}

	// Set the expected response status code
	expectedStatusCode := http.StatusOK

	// Set the expected response headers
	expectedHeaders := map[string]string{
		"Custom-Header": "Custom Value",
	}

	// Send the JSON response
	SendJSON(recorder, data, true, expectedStatusCode, expectedHeaders)

	// Get the actual response from the recorder
	response := recorder.Result()

	// Assert the response status code
	assert.Equal(t, expectedStatusCode, response.StatusCode, "Response status code should match")

	// Assert the response headers
	for key, value := range expectedHeaders {
		assert.Equal(t, value, response.Header.Get(key), "Response header value should match")
	}

	// Read the response body
	actualResponseBody, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err, "Failed to read response body")

	// Close the response body
	response.Body.Close()

	// Decode the response body into a Response struct
	var actualResponse Response
	err = json.Unmarshal(actualResponseBody, &actualResponse)
	assert.NoError(t, err, "Failed to unmarshal response body")

	// Assert the response data and success flag
	assert.Equal(t, data, actualResponse.Data, "Response data should match")
	assert.True(t, actualResponse.Success, "Response success flag should be true")
}

func TestValidateQueryString(t *testing.T) {
	// Create a new request with a query string parameter
	req := &http.Request{
		URL: &url.URL{
			RawQuery: "param=value",
		},
	}

	// Define the expected query string parameter name
	expectedQueryParam := "param"

	// Call the ValidateQueryString function
	value, err := ValidateQueryString(expectedQueryParam, req)

	// Assert that no error occurred
	assert.NoError(t, err, "No error should occur")

	// Assert that the returned value matches the expected value
	assert.Equal(t, "value", value, "Returned value should match the query string value")

	// Create a new request without the query string parameter
	reqWithoutParam := &http.Request{
		URL: &url.URL{},
	}

	// Call the ValidateQueryString function without the query string parameter
	_, err = ValidateQueryString(expectedQueryParam, reqWithoutParam)

	// Assert that an error occurred
	assert.Error(t, err, "An error should occur when query string parameter is missing")
}

func TestExtractBody(t *testing.T) {
	// Create a sample request with a request body
	bodyData := []byte(`{"key": "value"}`)
	req := httptest.NewRequest(http.MethodPost, "/path", bytes.NewBuffer(bodyData))
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder to capture the response
	resRecorder := httptest.NewRecorder()

	// Call the ExtractBody function
	body := ExtractBody(resRecorder, req)

	// Assert that the body is not nil
	assert.NotNil(t, body, "Body should not be nil")

	// Assert that the extracted body matches the expected body
	assert.Equal(t, bodyData, body, "Extracted body should match the request body")

	// Create a sample request without a request body
	emptyReq := httptest.NewRequest(http.MethodGet, "/path", nil)

	// Call the ExtractBody function with an empty request
	emptyBody := ExtractBody(resRecorder, emptyReq)

	// Assert that the empty body is nil
	assert.Nil(t, emptyBody, "Empty body should be nil")
}

func TestRetryOnError(t *testing.T) {
	// Define a callback function that returns an error
	callback := func() error {
		return errors.New("some error")
	}

	// Define the maximum number of retries and delay between retries
	maxRetry := uint64(3)
	delay := uint64(1) // in seconds

	// Call the RetryOnError function with the callback function
	err := RetryOnError(callback, maxRetry, delay)

	// Assert that the error is not nil
	assert.NotNil(t, err, "Error should not be nil")

	// Define a callback function that returns nil after the third retry
	callback = func() error {
		if maxRetry > 0 {
			maxRetry--
			return errors.New("some error")
		}
		return nil
	}

	// Call the RetryOnError function with the callback function
	err = RetryOnError(callback, maxRetry, delay)

	// Assert that the error is nil
	assert.Nil(t, err, "Error should be nil")
}
