package credential

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/service"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"io/ioutil"
	"net/http"
	"strconv"
)

type CredentialController controllers.Controller

func (credentialController *CredentialController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.SendJson(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return
	}

	if len(body) < 1 {
		utils.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := transformers.Credential{}

	err = credentialBody.FromJson(body)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}

	if newCredentialUUID, err := credentialService.CreateNewCredential(credentialBody.HTTPReferrerRestriction); err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
	} else {
		if credential, err := credentialService.FindOneCredentialByUUID(newCredentialUUID); err != nil {
			fmt.Println(err,newCredentialUUID)
			utils.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
		} else {
			utils.SendJson(w, credential, true, http.StatusCreated, nil)
		}
	}
}

func (credentialController *CredentialController) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.FindOneCredentialByUUID(params["uuid"])

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (credentialController *CredentialController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	if len(body) < 1 {
		utils.SendJson(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := transformers.Credential{}

	err = credentialBody.FromJson(body)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.UpdateOneCredential(params["uuid"], credentialBody.HTTPReferrerRestriction)

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusOK, nil)
	} else {
		utils.SendJson(w, credential, true, http.StatusOK, nil)
	}
}

func (credentialController *CredentialController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}
	_, err := credentialService.DeleteOneCredential(params["uuid"])
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusOK, nil)
	} else {
		utils.SendJson(w, nil, true, http.StatusNoContent, nil)
	}
}

func (credentialController *CredentialController) List(w http.ResponseWriter, r *http.Request) {
	credentialService := service.CredentialService{Pool: credentialController.Pool, Ctx: r.Context()}

	offset := 0
	limit := 50

	orderBy := "date_created DESC"

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err = strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err = strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	credentials, err := credentialService.ListCredentials(offset, limit, orderBy)

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	} else {
		utils.SendJson(w, credentials, true, http.StatusOK, nil)
		return
	}
}
