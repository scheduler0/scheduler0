package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/service/account"
	"scheduler0-private/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
)

type accountController struct {
	accountService account.AccountService
	logger         *log.Logger
}

func NewAccountController(logger *log.Logger, accountService account.AccountService) *accountController {
	return &accountController{
		accountService: accountService,
		logger:         logger,
	}
}

func (c *accountController) CreateOneAccount(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount entry", r.URL.Path))

	account := &models.Account{}
	err := json.NewDecoder(r.Body).Decode(account)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount error: failed to decode request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	accountId, createErr := c.accountService.CreateAccount(account)
	if createErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount error: failed to create account, error=%s", r.URL.Path, createErr.Message))
		utils.SendJSON(w, createErr, false, createErr.Type, nil)
		return
	}

	// Fetch the complete account object to return
	createdAccount, getErr := c.accountService.GetAccount(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount error: failed to get created account, accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	// Get features for the account
	features, getFeaturesErr := c.accountService.GetFeatures(accountId)
	if getFeaturesErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount error: failed to get features, accountId=%d, error=%s", r.URL.Path, accountId, getFeaturesErr.Message))
		utils.SendJSON(w, getFeaturesErr, false, getFeaturesErr.Type, nil)
		return
	}
	createdAccount.Features = *features

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneAccount success, status=201, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, createdAccount, true, http.StatusCreated, nil)
}

func (c *accountController) GetOneAccount(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneAccount error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneAccount entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	account, getErr := c.accountService.GetAccount(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneAccount error: failed to get account, accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	features, getFeaturesErr := c.accountService.GetFeatures(accountId)
	if getFeaturesErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneAccount error: failed to get features, accountId=%d, error=%s", r.URL.Path, accountId, getFeaturesErr.Message))
		utils.SendJSON(w, getFeaturesErr, false, getFeaturesErr.Type, nil)
		return
	}

	account.Features = *features

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetOneAccount success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, account, true, http.StatusOK, nil)
}

func (c *accountController) UpdateOneAccount(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount error: failed to decode request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}

	if updateErr := c.accountService.UpdateAccount(accountId, body.Name); updateErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount error: failed to update account, accountId=%d, error=%s", r.URL.Path, accountId, updateErr.Message))
		utils.SendJSON(w, updateErr, false, updateErr.Type, nil)
		return
	}

	account, getErr := c.accountService.GetAccount(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount error: failed to get updated account, accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	features, getFeaturesErr := c.accountService.GetFeatures(accountId)
	if getFeaturesErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount error: failed to get features, accountId=%d, error=%s", r.URL.Path, accountId, getFeaturesErr.Message))
		utils.SendJSON(w, getFeaturesErr, false, getFeaturesErr.Type, nil)
		return
	}
	account.Features = *features

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneAccount success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, account, true, http.StatusOK, nil)
}

func (c *accountController) AddFeature(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddFeature error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}
	featureRequest := models.FeatureRequest{}
	err = json.NewDecoder(r.Body).Decode(&featureRequest)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddFeature error: failed to decode request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddFeature entry, accountId=%d, featureId=%d", r.URL.Path, accountId, featureRequest.FeatureId))

	addFeatureErr := c.accountService.AddFeature(accountId, featureRequest.FeatureId)
	if addFeatureErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddFeature error: failed to add feature, accountId=%d, featureId=%d, error=%s", r.URL.Path, accountId, featureRequest.FeatureId, addFeatureErr.Message))
		utils.SendJSON(w, addFeatureErr, false, addFeatureErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddFeature success, status=201, accountId=%d, featureId=%d", r.URL.Path, accountId, featureRequest.FeatureId))
	utils.SendJSON(w, featureRequest, true, http.StatusCreated, nil)
}

func (c *accountController) RemoveFeature(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveFeature error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}
	featureRequest := models.FeatureRequest{}
	err = json.NewDecoder(r.Body).Decode(&featureRequest)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveFeature error: failed to decode request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveFeature entry, accountId=%d, featureId=%d", r.URL.Path, accountId, featureRequest.FeatureId))

	removeFeatureErr := c.accountService.RemoveFeature(accountId, featureRequest.FeatureId)
	if removeFeatureErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveFeature error: failed to remove feature, accountId=%d, featureId=%d, error=%s", r.URL.Path, accountId, featureRequest.FeatureId, removeFeatureErr.Message))
		utils.SendJSON(w, removeFeatureErr, false, removeFeatureErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveFeature success, status=204, accountId=%d, featureId=%d", r.URL.Path, accountId, featureRequest.FeatureId))
	utils.SendJSON(w, featureRequest, true, http.StatusNoContent, nil)
}

func (c *accountController) AddAllFeatures(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddAllFeatures error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddAllFeatures entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	addAllFeaturesErr := c.accountService.AddAllFeatures(accountId)
	if addAllFeaturesErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddAllFeatures error: failed to add all features, accountId=%d, error=%s", r.URL.Path, accountId, addAllFeaturesErr.Message))
		utils.SendJSON(w, addAllFeaturesErr, false, addAllFeaturesErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddAllFeatures success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, map[string]string{"message": "All features added successfully"}, true, http.StatusOK, nil)
}

func (c *accountController) RemoveAllFeatures(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveAllFeatures error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveAllFeatures entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	removeAllFeaturesErr := c.accountService.RemoveAllFeatures(accountId)
	if removeAllFeaturesErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveAllFeatures error: failed to remove all features, accountId=%d, error=%s", r.URL.Path, accountId, removeAllFeaturesErr.Message))
		utils.SendJSON(w, removeAllFeaturesErr, false, removeAllFeaturesErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("DELETE %s - RemoveAllFeatures success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, map[string]string{"message": "All features removed successfully"}, true, http.StatusOK, nil)
}

func (c *accountController) GetExecutionCount(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetExecutionCount error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetExecutionCount entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	executionCount, getErr := c.accountService.GetExecutionCount(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetExecutionCount error: failed to get execution count, accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetExecutionCount success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, executionCount, true, http.StatusOK, nil)
}

// GetTokens returns the current platform-token balance for an account.
func (c *accountController) GetTokens(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetTokens error: invalid account ID, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	tokens, getErr := c.accountService.GetTokens(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetTokens error: accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetTokens success, accountId=%d, tokens=%d", r.URL.Path, accountId, tokens))
	utils.SendJSON(w, map[string]uint64{"tokens": tokens}, true, http.StatusOK, nil)
}

// AddTokens adds platform tokens to an account's balance.
// Body: { "amount": N }
func (c *accountController) AddTokens(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - AddTokens error: invalid account ID, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	req := struct {
		Amount uint64 `json:"amount"`
	}{}
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - AddTokens error: failed to decode body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	if req.Amount == 0 {
		utils.SendJSON(w, "amount must be greater than zero", false, http.StatusBadRequest, nil)
		return
	}

	newBalance, addErr := c.accountService.AddTokens(accountId, req.Amount)
	if addErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - AddTokens error: accountId=%d, error=%s", r.URL.Path, accountId, addErr.Message))
		utils.SendJSON(w, addErr, false, addErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - AddTokens success, accountId=%d, added=%d, newBalance=%d", r.URL.Path, accountId, req.Amount, newBalance))
	utils.SendJSON(w, map[string]uint64{"newBalance": newBalance}, true, http.StatusOK, nil)
}

// IncreaseExecutionCount increases the execution count for an account
func (c *accountController) IncreaseExecutionCount(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - IncreaseExecutionCount error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}
	countRequest := struct {
		Count uint64 `json:"count"`
	}{}
	err = json.NewDecoder(r.Body).Decode(&countRequest)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - IncreaseExecutionCount error: failed to decode request body, accountId=%d, error=%v", r.URL.Path, accountId, err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}

	newExecutionCount, increaseExecutionCountErr := c.accountService.IncreaseExecutionCount(accountId, countRequest.Count)
	if increaseExecutionCountErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - IncreaseExecutionCount error: failed to increase execution count, accountId=%d, error=%s", r.URL.Path, accountId, increaseExecutionCountErr.Message))
		utils.SendJSON(w, increaseExecutionCountErr, false, increaseExecutionCountErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("PUT %s - IncreaseExecutionCount success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, map[string]uint64{"newExecutionCount": newExecutionCount}, true, http.StatusOK, nil)
}

// enforceAccountScope closes cross-account reads/writes on /accounts/{id} endpoints.
// When an authenticated account is present in the request context (set from the X-Account-ID
// header) it must match the account id in the path; otherwise the request is rejected with
// 403. Requests without a context account (trusted peer/admin calls such as billing top-ups)
// are allowed through unchanged, preserving existing server-to-server behavior.
func (c *accountController) enforceAccountScope(w http.ResponseWriter, r *http.Request, requestID string, pathAccountId uint64) bool {
	ctxAccountId, ok := utils.GetAccountID(r.Context())
	if ok && ctxAccountId != pathAccountId {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("%s %s - account scope violation: context account %d attempted to access account %d", r.Method, r.URL.Path, ctxAccountId, pathAccountId))
		utils.SendJSON(w, "account mismatch: cannot access another account", false, http.StatusForbidden, nil)
		return false
	}
	return true
}

// GetAIUsage returns the account's log-derived AI request usage for the current period:
// prompt and classify limits (feature-derived), the number of successful requests used,
// the remaining allowance, the sum of estimated prompt cost USD, and the period boundary.
// It is the single source of truth the dashboard renders and request handlers enforce against.
func (c *accountController) GetAIUsage(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAIUsage error: invalid account ID parameter, id=%s, error=%v", r.URL.Path, params["id"], err))
		utils.SendJSON(w, err, false, http.StatusBadRequest, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAIUsage entry, accountId=%d", r.URL.Path, accountId))

	if !c.enforceAccountScope(w, r, requestID, accountId) {
		return
	}

	usage, getErr := c.accountService.GetAIUsage(accountId)
	if getErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAIUsage error: failed to get ai usage, accountId=%d, error=%s", r.URL.Path, accountId, getErr.Message))
		utils.SendJSON(w, getErr, false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetAIUsage success, status=200, accountId=%d", r.URL.Path, accountId))
	utils.SendJSON(w, usage, true, http.StatusOK, nil)
}
