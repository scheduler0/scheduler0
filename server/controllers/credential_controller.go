package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/migrations"
	"cron-server/server/service"
	"io/ioutil"
	"net/http"
)

type CredentialController struct {
	Pool migrations.Pool
}

func (cc *CredentialController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	if len(body) < 1 {
		misc.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
	}

	var credentialBody models.Credential

	err = credentialBody.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
	}

	var credentialService = service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	if newCredentialID, err := credentialService.CreateNewCredential(credentialBody.HTTPReferrerRestriction); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusTeapot, nil)
	} else {
		misc.SendJson(w, newCredentialID, true, http.StatusCreated, nil)
	}
}

func (cc *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
}

func (cc *CredentialController) List(w http.ResponseWriter, r *http.Request) {
}