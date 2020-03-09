package controllers

import (
	"cron-server/server/dtos"
	"cron-server/server/migrations"
	"cron-server/server/misc"
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

	credentialBody := dtos.CredentialDto{}

	err = credentialBody.FromJson(body)
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
	}

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }

	if newCredentialID, err := credentialService.CreateNewCredential(credentialBody.HTTPReferrerRestriction); err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusTeapot, nil)
	} else {
		misc.SendJson(w, newCredentialID, true, http.StatusCreated, nil)
	}
}

func (cc *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	id, err := misc.GetRequestParam(r, "id", 2)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.FindOneCredentialByID(id)

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	}

	misc.SendJson(w, credential, true, http.StatusOK, nil)
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
}

func (cc *CredentialController) List(w http.ResponseWriter, r *http.Request) {
}