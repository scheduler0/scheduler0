package controllers

import (
	"cron-server/server/src/controllers"
	"cron-server/server/src/misc"
	"cron-server/server/src/transformers"
	"cron-server/server/tests"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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

			body, err := ioutil.ReadAll(w.Body)
			if err != nil {
				fmt.Print(err)
			}

			res := &misc.Response{}

			err = json.Unmarshal(body, res)
			if err != nil {
				fmt.Print(err)
			}

			if !assert.Equal(t, http.StatusCreated, w.Code) {
				fmt.Println("Server response body", res)
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

			body, err := ioutil.ReadAll(w.Body)
			if err != nil {
				fmt.Print(err)
			}

			res := &misc.Response{}

			err = json.Unmarshal(body, res)
			if err != nil {
				fmt.Print(err)
			}

			if !assert.Equal(t, http.StatusOK, w.Code) {
				fmt.Errorf("server response error body %v", res)
			} else {
				// Uncomment to view response body in logs
				// fmt.Println(string(body))
			}
		}

		t.Logf("\t\tUpdate A Credential")
		{

		}
	}
}
