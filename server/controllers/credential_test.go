package controllers

import (
	"cron-server/server/dtos"
	"cron-server/server/migrations"
	"cron-server/server/misc"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCredentialController_CreateOne(t *testing.T) {
	t.Log("Creating A New Credential")
	{
		t.Logf("")
		{
			pool, err := migrations.NewPool(migrations.CreateConnection, 1)
			credentialController := CredentialController{}
			misc.CheckErr(err)
			credentialController.Pool = *pool

			testCredential := dtos.CredentialDto{  HTTPReferrerRestriction: "*" }

			jsonTestCredentialBody, err := testCredential.ToJson()
			if err != nil {
				t.Fatalf("\t\t Cannot create http request %v", err)
			}

			req, err := http.NewRequest("POST", "/", strings.NewReader(string(jsonTestCredentialBody)))

			if err != nil {
				t.Fatalf("\t\t Cannot create http request %v", err)
			}

			w := httptest.NewRecorder()
			credentialController.ListAll(w, req)

			body, err := ioutil.ReadAll(w.Body)
			if err != nil {
				fmt.Print(err)
			}

			var res misc.Response

			err = json.Unmarshal(body, res)
			if err != nil {
				fmt.Print(err)
			}

			assert.Equal(t, http.StatusOK, w.Code)
		}
	}
}
