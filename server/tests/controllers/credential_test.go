package controllers

import (
	"cron-server/server/src/controllers"
	"cron-server/server/src/transformers"
	"cron-server/server/tests"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


func TestCredential_Controller(t *testing.T) {

	pool := tests.GetTestPool()
	credentialController := controllers.CredentialController{ Pool: pool }

	t.Log("TestCredential_Controller")
	{

		t.Logf("\t\tCreating A New Credential")
		{
			testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
			jsonTestCredentialBody, err := testCredential.ToJson()
			if err != nil {
				t.Fatalf("\t\t Cannot create http request %v", err)
			}

			req, err := http.NewRequest("POST", "/", strings.NewReader(string(jsonTestCredentialBody)))
			if err != nil {
				t.Fatalf("\t\t Cannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			credentialController.CreateOne(w, req)
			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusCreated, w.Code) {
				fmt.Errorf("server response body %v", res)
			} else {
				// Uncomment to view response body in logs
				//fmt.Println(responseStr)
			}
		}

		t.Logf("\t\tGet All Credentials")
		{
			req, err := http.NewRequest("GET", "/", nil)
			if err != nil {
				t.Fatalf("\t\t Cannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			credentialController.List(w, req)
			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusOK, w.Code) {
				fmt.Errorf("server response error body %v", res)
			} else {
				// Uncomment to view response body in logs
				// fmt.Println(responseStr)
			}
		}

		t.Logf("\t\tGet A Single Credential")
		{

		}
	}
}
