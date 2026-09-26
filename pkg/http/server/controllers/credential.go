package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/credential"
	"scheduler0/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
)

type CredentialHTTPController interface {
	CreateOneCredential(w http.ResponseWriter, r *http.Request)
	GetOneCredential(w http.ResponseWriter, r *http.Request)
	UpdateOneCredential(w http.ResponseWriter, r *http.Request)
	DeleteOneCredential(w http.ResponseWriter, r *http.Request)
	ArchiveOneCredential(w http.ResponseWriter, r *http.Request)
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

// requestGrantsAdminScope reports whether the requested scope set includes admin.
func requestGrantsAdminScope(scopes []string) bool {
	for _, s := range scopes {
		if s == constants.CredentialScopeAdmin {
			return true
		}
	}
	return false
}

// callerMayGrantAdmin reports whether the current request is permitted to mint an
// admin-scoped credential. A peer/operator request carries no credential in
// context (basic auth) and is allowed; an api-key request is allowed only when
// its own credential already holds the admin scope.
func callerMayGrantAdmin(r *http.Request) bool {
	callerCred, ok := r.Context().Value(utils.CredentialContextKey()).(*models.Credential)
	if !ok || callerCred == nil {
		// No api-key credential in context => peer/operator (basic auth).
		return true
	}
	return callerCred.HasScope(constants.CredentialScopeAdmin)
}

// CreateOneCredential CreateOne create a single credential
func (credentialController *credentialController) CreateOneCredential(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential entry", r.URL.Path))

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: failed to read request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return
	}

	if len(body) < 1 {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: empty request body", r.URL.Path))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	credentialBody := models.Credential{}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: failed to unmarshal request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if credentialBody.CreatedBy == "" {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: createdBy is required", r.URL.Path))
		utils.SendJSON(w, "createdBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	credentialBody.AccountId = accountId

	// Escalation guard: the admin scope can never be self-granted. A request may
	// mint an admin-scoped credential only when it comes from a peer/operator
	// (basic auth — no credential in context) or from an api-key caller that
	// already holds the admin scope.
	if requestGrantsAdminScope(credentialBody.Scopes) && !callerMayGrantAdmin(r) {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: caller not permitted to grant admin scope, accountId=%d", r.URL.Path, accountId))
		utils.SendJSON(w, "admin scope may only be granted by an operator or an admin credential", false, http.StatusForbidden, nil)
		return
	}

	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential processing, accountId=%d", r.URL.Path, accountId))

	if newCredentialUUID, plaintextSecret, err := credentialController.credentialService.CreateNewCredential(credentialBody); err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: failed to create credential, accountId=%d, error=%s", r.URL.Path, accountId, err.Message))
		utils.SendJSON(w, err.Message, false, err.Type, nil)
	} else {
		if credential, err := credentialController.credentialService.FindOneCredentialByID(newCredentialUUID, accountId); err != nil {
			utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: failed to get created credential, credentialUUID=%d, accountId=%d, error=%v", r.URL.Path, newCredentialUUID, accountId, err))
			utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		} else {
			utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential success, status=201, credentialId=%d, accountId=%d", r.URL.Path, credential.ID, accountId))
			// The plaintext secret is surfaced exactly once, here, under plaintextSecret. It is
			// never persisted in plaintext (the DB stores it encrypted) and the base Credential
			// model never serializes ApiSecret (json:"-"), so read endpoints don't leak it.
			response := struct {
				models.Credential
				PlaintextSecret string `json:"plaintextSecret,omitempty"`
			}{
				Credential:      *credential,
				PlaintextSecret: plaintextSecret,
			}
			utils.SendJSON(w, response, true, http.StatusCreated, nil)
		}
	}
}

// GetOneCredential GetOne returns a single credential
func (credentialController *credentialController) GetOneCredential(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	credentialService := credentialController.credentialService
	credentialId, err := strconv.Atoi(params["id"])
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneCredential error: invalid credential ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}
	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneCredential entry, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))

	credential, err := credentialService.FindOneCredentialByID(uint64(credentialId), accountId)

	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneCredential error: failed to get credential, credentialId=%d, accountId=%d, error=%v", r.URL.Path, credentialId, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
	} else {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - GetOneCredential success, status=200, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// UpdateOneCredential UpdateOne updates a single credential
func (credentialController *credentialController) UpdateOneCredential(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: failed to read request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}
	if len(body) < 1 {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: empty request body", r.URL.Path))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}
	credentialId, err := strconv.Atoi(params["id"])
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: invalid credential ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}
	credentialBody := models.Credential{
		ID: uint64(credentialId),
	}

	err = credentialBody.FromJSON(body)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: failed to unmarshal request body, credentialId=%d, error=%v", r.URL.Path, credentialId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	if credentialBody.ModifiedBy == nil || *credentialBody.ModifiedBy == "" {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: modifiedBy is required, credentialId=%d", r.URL.Path, credentialId))
		utils.SendJSON(w, "modifiedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	credentialBody.AccountId = accountId
	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential entry, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))

	credentialService := credentialController.credentialService
	credential, err := credentialService.UpdateOneCredential(credentialBody)

	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential error: failed to update credential, credentialId=%d, accountId=%d, error=%v", r.URL.Path, credentialId, accountId, err))
		status := http.StatusBadRequest
		message := err.Error()
		if genericErr, ok := err.(*utils.GenericError); ok {
			status = genericErr.Type
			message = genericErr.Message
		}
		utils.SendJSON(w, message, false, status, nil)
	} else {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneCredential success, status=200, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))
		utils.SendJSON(w, credential, true, http.StatusOK, nil)
	}
}

// DeleteOneCredential DeleteOne deletes a single credential
func (credentialController *credentialController) DeleteOneCredential(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)
	credentialService := credentialController.credentialService
	credentialId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential error: invalid credential ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}
	// Parse request body to get deletedBy
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential error: failed to read request body, credentialId=%d, error=%v", r.URL.Path, credentialId, readErr))
		utils.SendJSON(w, "Failed to read request body", false, http.StatusBadRequest, nil)
		return
	}

	var deleteRequest struct {
		DeletedBy string `json:"deletedBy"`
	}

	if err := json.Unmarshal(body, &deleteRequest); err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential error: invalid request body, credentialId=%d, error=%v", r.URL.Path, credentialId, err))
		utils.SendJSON(w, "Invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if deleteRequest.DeletedBy == "" {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential error: deletedBy is required, credentialId=%d", r.URL.Path, credentialId))
		utils.SendJSON(w, "deletedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential entry, credentialId=%d, accountId=%d, deletedBy=%s", r.URL.Path, credentialId, accountId, deleteRequest.DeletedBy))

	_, err := credentialService.DeleteOneCredential(uint64(credentialId), accountId, deleteRequest.DeletedBy)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential error: failed to delete credential, credentialId=%d, accountId=%d, error=%v", r.URL.Path, credentialId, accountId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	} else {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneCredential success, status=204, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))
		utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
		return
	}
}

// ArchiveOneCredential archives a single credential
func (credentialController *credentialController) ArchiveOneCredential(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	credentialId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential error: invalid credential ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, "credential id is required", false, http.StatusBadRequest, nil)
		return
	}

	// Parse request body to get archivedBy
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential error: failed to read request body, credentialId=%d, error=%v", r.URL.Path, credentialId, err))
		utils.SendJSON(w, "Failed to read request body", false, http.StatusBadRequest, nil)
		return
	}

	var archiveRequest struct {
		ArchivedBy string `json:"archivedBy"`
	}

	if err := json.Unmarshal(body, &archiveRequest); err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential error: invalid request body, credentialId=%d, error=%v", r.URL.Path, credentialId, err))
		utils.SendJSON(w, "Invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if archiveRequest.ArchivedBy == "" {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential error: archivedBy is required, credentialId=%d", r.URL.Path, credentialId))
		utils.SendJSON(w, "archivedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	credentialService := credentialController.credentialService

	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential entry, credentialId=%d, accountId=%d, archivedBy=%s", r.URL.Path, credentialId, accountId, archiveRequest.ArchivedBy))

	_, archiveErr := credentialService.ArchiveOneCredential(uint64(credentialId), accountId, archiveRequest.ArchivedBy)
	if archiveErr != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential error: failed to archive credential, credentialId=%d, accountId=%d, error=%s", r.URL.Path, credentialId, accountId, archiveErr.Message))
		utils.SendJSON(w, archiveErr.Error(), false, archiveErr.Type, nil)
		return
	}

	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - ArchiveOneCredential success, status=204, credentialId=%d, accountId=%d", r.URL.Path, credentialId, accountId))
	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

// ListCredentials List returns a paginated list of credentials
func (credentialController *credentialController) ListCredentials(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials entry, query=%s", r.URL.Path, r.URL.RawQuery))

	credentialService := credentialController.credentialService

	offset := 0
	limit := 0

	defaultLimit := strconv.Itoa(constants.DefaultListLimit)
	limitParam, err := utils.ValidateQueryStringWithDefault("limit", r, &defaultLimit)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	defaultOffset := "0"
	offsetParam, err := utils.ValidateQueryStringWithDefault("offset", r, &defaultOffset)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err = strconv.Atoi(offsetParam)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: invalid offset parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err = strconv.Atoi(limitParam)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: invalid limit parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	defaultOrderByColumn := constants.CredentialsDateCreatedColumn
	defaultOrderByDirection := constants.OrderDirectionDesc

	orderByColumn, err := utils.ValidateQueryStringWithDefault("orderBy", r, &defaultOrderByColumn)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	orderByDirection, err := utils.ValidateQueryStringWithDefault("orderByDirection", r, &defaultOrderByDirection)
	if err != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneCredential error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	credentials, listCredentialError := credentialService.ListCredentials(uint64(offset), uint64(limit), orderByColumn, orderByDirection, accountId)

	if listCredentialError != nil {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials error: failed to list credentials, accountId=%d, offset=%d, limit=%d, error=%s", r.URL.Path, accountId, offset, limit, listCredentialError.Message))
		utils.SendJSON(w, listCredentialError.Message, false, listCredentialError.Type, nil)
		return
	} else {
		utils.LogWithRequestID(credentialController.logger, requestID, "", fmt.Sprintf("GET %s - ListCredentials success, status=200, accountId=%d, count=%d, offset=%d, limit=%d", r.URL.Path, accountId, credentials.Total, offset, limit))
		utils.SendJSON(w, credentials, true, http.StatusOK, nil)
		return
	}
}
