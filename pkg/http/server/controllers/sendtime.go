package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/service/sendtime"
	"scheduler0-private/pkg/utils"
)

// SendTimeHTTPController exposes the deterministic send-time suggestion endpoint.
type SendTimeHTTPController interface {
	SendTimeSuggestions(w http.ResponseWriter, r *http.Request)
}

type sendTimeController struct {
	sendTimeService sendtime.SendTimeService
	logger          *log.Logger
}

// NewSendTimeController builds the send-time suggestions controller.
func NewSendTimeController(logger *log.Logger, sendTimeService sendtime.SendTimeService) SendTimeHTTPController {
	return &sendTimeController{
		sendTimeService: sendTimeService,
		logger:          logger,
	}
}

func (c *sendTimeController) SendTimeSuggestions(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions entry", r.URL.Path)

	body := utils.ExtractBody(w, r)
	if body == nil {
		utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions error: empty request body", r.URL.Path)
		return
	}

	var req models.SendTimeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions error: unmarshal, error=%v", r.URL.Path, err)
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	// Structured input validation (invalid timezone, search window, interval, etc.).
	if verr := c.sendTimeService.Validate(req); verr != nil {
		utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions validation error: %s %s", r.URL.Path, verr.Code, verr.Message)
		data := map[string]any{"code": verr.Code, "message": verr.Message}
		if verr.Field != "" {
			data["field"] = verr.Field
		}
		utils.SendJSON(w, data, false, verr.Status, nil)
		return
	}

	// The endpoint is account-scoped (see AccountIDMiddleware); require the context value.
	if _, ok := utils.GetAccountID(r.Context()); !ok {
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	result, err := c.sendTimeService.Suggest(req)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions engine error: %v", r.URL.Path, err)
		utils.SendJSON(w, map[string]any{
			"code":       "SUGGESTION_ENGINE_FAILURE",
			"message":    "The suggestion engine could not evaluate this request.",
			"request_id": requestID,
		}, false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "POST %s - SendTimeSuggestions success, suggestions=%d", r.URL.Path, len(result.Suggestions))
	utils.SendJSON(w, result, true, http.StatusOK, nil)
}
