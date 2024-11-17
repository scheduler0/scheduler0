package controllers

import (
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/credential"
	"scheduler0/pkg/utils"
	"strconv"
)

type CredentialHTTPController interface {
	CreateOneCredential(w http.ResponseWriter, r *http.Request)
	GetOneCredential(w http.ResponseWriter, r *http.Request)
	UpdateOneCredential(w http.ResponseWriter, r *http.Request)
	DeleteOneCredential(w http.ResponseWriter, r *http.Request)
	ListCredentials(w http.ResponseWriter, r *http.Request)
}

type credentialController struct {
	credentialService credential.CredentialService
	logger            *log.Logger
}

func NewCredentialController(logger *log.Logger, credentialService credential.CredentialService) CredentialHTTPController {
	return &credentialController{
		credentialService: credentialService,
		logger:            logger,
	}
}

// CreateOneCredential CreateOne create a single credential
func (credentialController *credentialController) CreateOneCredential(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.SendJSON(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return
	}

	if len(body) < 1 {
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := models.Credential{}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if newCredentialUUID, err := credentialController.credentialService.CreateNewCredential(credentialBody); err != nil {
		utils.SendJSON(w, err.Message, false, err.Type, nil)
	} else {
		if credential, err := credentialController.credentialService.FindOneCredentialByID(newCredentialUUID); err != nil {
			credentialController.logger.Println(err, newCredentialUUID)
			utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		} else {
			utils.SendJSON(w, credential, true, http.StatusCreated, nil)
		}
	}
}

// GetOneCredential GetOne returns a single credential
func (credentialController *credentialController) GetOneCredential(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	credentialService := credentialController.credentialService
	credentialId, err := strconv.Atoi(params["id"])
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}
	credential, err := credentialService.FindOneCredentialByID(uint64(credentialId))

	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// UpdateOneCredential UpdateOne updates a single credential
func (credentialController *credentialController) UpdateOneCredential(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		credentialController.logger.Fatalln(err)
	}
	if len(body) < 1 {
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}
	credentialId, err := strconv.Atoi(params["id"])
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}
	credentialBody := models.Credential{
		ID: uint64(credentialId),
	}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	credentialService := credentialController.credentialService
	credential, err := credentialService.UpdateOneCredential(credentialBody)

	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusOK, nil)
	} else {
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// DeleteOneCredential DeleteOne deletes a single credential
func (credentialController *credentialController) DeleteOneCredential(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	credentialService := credentialController.credentialService
	credentialId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}
	_, err := credentialService.DeleteOneCredential(uint64(credentialId))
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	} else {
		utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
		return
	}
}

// ListCredentials List returns a paginated list of credentials
func (credentialController *credentialController) ListCredentials(w http.ResponseWriter, r *http.Request) {
	credentialService := credentialController.credentialService

	offset := 0
	limit := 50

	// TODO: use constants for ASC and DESC
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

	credentials, listCredentialError := credentialService.ListCredentials(uint64(offset), uint64(limit), orderBy)

	if listCredentialError != nil {
		utils.SendJSON(w, listCredentialError.Message, false, listCredentialError.Type, nil)
		return
	} else {
		utils.SendJSON(w, credentials, true, http.StatusOK, nil)
		return
	}
}
