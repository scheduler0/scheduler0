package controllers

import (
	"cron-server/server/src/controllers"
	"cron-server/server/src/transformers"
	"cron-server/server/tests"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


func TestCredential_Controller(t *testing.T) {

	pool := tests.GetTestPool()
	credentialController := controllers.CredentialController{ Pool: pool }
	credential := transformers.Credential{}

	t.Log("TestCredential_Controller")
	{
		t.Logf("\t\tCreating A New Credential")
		{
			testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
			jsonTestCredentialBody, err := testCredential.ToJson()
			if err != nil {
				t.Errorf("\t\tcannot create http request %v", err)
			}

			req, err := http.NewRequest("POST", "/", strings.NewReader(string(jsonTestCredentialBody)))
			if err != nil {
				t.Errorf("\t\tcannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			credentialController.CreateOne(w, req)
			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusCreated, w.Code) {
				t.Errorf("server response body %v", res)
			} else {
				// Uncomment to view response body in logs
				newCredentialMap := res.Data.(map[string]interface {})
				if credentialByte, err := json.Marshal(newCredentialMap); err != nil {
					t.Errorf("failed to encode interface to json %v", err)
				} else {
					if err := json.Unmarshal(credentialByte, &credential); err != nil {
						t.Errorf("failed to convert byte into credential %v", err)
					}
				}
			}
		}

		t.Logf("\t\tGet All Credentials")
		{
			req, err := http.NewRequest("GET", "/?limit=50&offset=0", nil)
			if err != nil {
				t.Errorf("\t\t cannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			credentialController.List(w, req)
			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusOK, w.Code) {
				t.Errorf("server response error body %v", res)
			}
		}

		t.Logf("\t\tGet One Credential")
		{
			path := fmt.Sprintf("/credentials/%s", credential.ID)
			req, err := http.NewRequest("GET", path, nil)
			if err != nil {
				t.Errorf("\t\t cannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/credentials/{id}", credentialController.GetOne)
			router.ServeHTTP(w, req)

			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusOK, w.Code) {
				t.Errorf("server response error body %v", res)
			}
		}

		t.Logf("\t\tUpdate One Credential")
		{

			newHTTPReferrerRestriction :=  "http://scheduler0.com"
			updateBody := transformers.Credential{HTTPReferrerRestriction: newHTTPReferrerRestriction}
			jsonTestCredentialBody, err := updateBody.ToJson()
			if err != nil {
				t.Errorf("\t\tcannot create http request %v", err)
			}

			path := fmt.Sprintf("/credentials/%s", credential.ID)
			req, err := http.NewRequest("PUT", path, strings.NewReader(string(jsonTestCredentialBody)))
			if err != nil {
				t.Errorf("\t\tcannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/credentials/{id}", credentialController.UpdateOne)
			router.ServeHTTP(w, req)

			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusOK, w.Code) {
				t.Errorf("server response error body %v", res)
			} else {
				newCredentialMap := res.Data.(map[string]interface {})
				if credentialByte, err := json.Marshal(newCredentialMap); err != nil {
					t.Errorf("failed to encode interface to json %v", err)
				} else {
					if err := json.Unmarshal(credentialByte, &credential); err != nil {
						t.Errorf("failed to convert byte into credential %v", err)
					} else {
						if credential.HTTPReferrerRestriction != newHTTPReferrerRestriction {
							t.Errorf("expected new HTTPReferrerRestriction to be %v but got %v", newHTTPReferrerRestriction, credential.HTTPReferrerRestriction)
						}
					}
				}
			}
		}

		t.Logf("\t\tDelete One Credential")
		{
			path := fmt.Sprintf("/credentials/%s", credential.ID)
			req, err := http.NewRequest("DELETE", path, nil)
			if err != nil {
				t.Errorf("\t\tcannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/credentials/{id}", credentialController.DeleteOne)
			router.ServeHTTP(w, req)

			res, _, err := tests.ExtractResponse(w)

			if !assert.Equal(t, http.StatusNoContent, w.Code) {
				t.Errorf("server response error body %v", res)
			}
		}
	}
}
