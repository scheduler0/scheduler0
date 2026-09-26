package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"scheduler0-private/pkg/service/secret_rotation"
	"scheduler0-private/pkg/utils"
)

type SecretRotationHTTPController interface {
	RotateSecret(w http.ResponseWriter, r *http.Request)
}

type secretRotationController struct {
	service secret_rotation.SecretRotationService
	logger  *log.Logger
}

func NewSecretRotationController(logger *log.Logger, service secret_rotation.SecretRotationService) SecretRotationHTTPController {
	return &secretRotationController{
		service: service,
		logger:  logger,
	}
}

// RotateSecret re-encrypts all secrets stored under the server SecretKey (executor cloud
// credentials and per-account AI provider keys) from the supplied old key to the server's
// currently-loaded SecretKey. This endpoint is peer/basic-auth only (see AuthMiddleware).
func (c *secretRotationController) RotateSecret(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret entry", r.URL.Path))

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret error: failed to read request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}
	if len(body) < 1 {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret error: empty request body", r.URL.Path))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	var reqBody struct {
		OldSecretKey string `json:"oldSecretKey"`
	}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret error: invalid request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	result, rotateErr := c.service.RotateSecret(reqBody.OldSecretKey)
	if rotateErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret error: %s", r.URL.Path, rotateErr.Message))
		utils.SendJSON(w, rotateErr.Message, false, rotateErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RotateSecret success, status=200, credentialsRotated=%d, executorsRotated=%d, aiSettingsRotated=%d", r.URL.Path, result.CredentialsRotated, result.ExecutorsRotated, result.AISettingsRotated))
	utils.SendJSON(w, result, true, http.StatusOK, nil)
}
