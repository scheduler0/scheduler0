package credential

import (
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"scheduler0/server/http_server/controllers"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strconv"
)

// Controller Handle all requests to /credentials
type Controller controllers.Controller

// CreateOne create a single credential
func (credentialController *Controller) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.SendJSON(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return
	}

	if len(body) < 1 {
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := transformers.Credential{}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	credentialService := service.Credential{Pool: credentialController.Pool, Ctx: r.Context()}

	if newCredentialUUID, err := credentialService.CreateNewCredential(credentialBody); err != nil {
		utils.SendJSON(w, err.Message, false, err.Type, nil)
	} else {
		if credential, err := credentialService.FindOneCredentialByUUID(newCredentialUUID); err != nil {
			fmt.Println(err, newCredentialUUID)
			utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		} else {
			utils.SendJSON(w, credential, true, http.StatusCreated, nil)
		}
	}
}

// GetOne returns a single credential
func (credentialController *Controller) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := service.Credential{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.FindOneCredentialByUUID(params["uuid"])

	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// UpdateOne updates a single credential
func (credentialController *Controller) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	if len(body) < 1 {
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := transformers.Credential{
		UUID: params["uuid"],
	}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	credentialService := service.Credential{Pool: credentialController.Pool, Ctx: r.Context()}
	credential, err := credentialService.UpdateOneCredential(credentialBody)

	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusOK, nil)
	} else {
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// DeleteOne deletes a single credential
func (credentialController *Controller) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := service.Credential{Pool: credentialController.Pool, Ctx: r.Context()}
	_, err := credentialService.DeleteOneCredential(params["uuid"])
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
	}
}

// List returns a paginated list of credentials
func (credentialController *Controller) List(w http.ResponseWriter, r *http.Request) {
	credentialService := service.Credential{Pool: credentialController.Pool, Ctx: r.Context()}

	offset := 0
	limit := 50

	orderBy := "date_created DESC"

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err = strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err = strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	credentials, listCredentialError := credentialService.ListCredentials(offset, limit, orderBy)

	if listCredentialError != nil {
		utils.SendJSON(w, listCredentialError.Message, false, listCredentialError.Type, nil)
		return
	} else {
		utils.SendJSON(w, credentials, true, http.StatusOK, nil)
		return
	}
}
