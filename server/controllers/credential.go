package controllers

import (
	"cron-server/server/dtos"
	"cron-server/server/migrations"
	"cron-server/server/misc"
	"cron-server/server/service"
	"github.com/gorilla/mux"
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
	params := mux.Vars(r)

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.FindOneCredentialByID(params["id"])

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.UpdateOneCredential(params["id"])

	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credential, err := credentialService.DeleteOneCredential(params["id"])
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (cc *CredentialController) List(w http.ResponseWriter, r *http.Request) {
	credentialService := service.CredentialService{ Pool: cc.Pool, Ctx: r.Context() }
	credentials, err := credentialService.ListCredentials()
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusOK, nil)
		return
	} else {
		misc.SendJson(w, credentials, true, http.StatusOK, nil)
	}
}