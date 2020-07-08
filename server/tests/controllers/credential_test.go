package controllers

import (
	"cron-server/server/src/controllers"
	"cron-server/server/src/db"
	"cron-server/server/src/misc"
	"cron-server/server/src/transformers"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


func TestCredential_Controller(t *testing.T) {
	t.Log("Creating A New Credential")
	{
		pool, err := misc.NewPool(func() (closer io.Closer, err error) {
			return db.CreateConnectionEnv("TEST")
		}, 1)
		credentialController := controllers.CredentialController{}
		misc.CheckErr(err)
		credentialController.Pool = pool

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
}
