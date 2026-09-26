package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0-private/pkg/models"
	ai_credits_svc "scheduler0-private/pkg/service/ai_credits"
	"scheduler0-private/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
)

type AICreditsInternalController interface {
	GetCredits(w http.ResponseWriter, r *http.Request)
	Credit(w http.ResponseWriter, r *http.Request)
	UpdateAutoTopup(w http.ResponseWriter, r *http.Request)
	GetLedger(w http.ResponseWriter, r *http.Request)
}

type aiCreditsInternalController struct {
	creditsService ai_credits_svc.AICreditsService
	logger         *log.Logger
}

func NewAICreditsInternalController(logger *log.Logger, creditsService ai_credits_svc.AICreditsService) AICreditsInternalController {
	return &aiCreditsInternalController{creditsService: creditsService, logger: logger}
}

type creditsResponse struct {
	AccountID                uint64  `json:"account_id"`
	BalanceMicros            int64   `json:"balance_micros"`
	BalanceUSD               float64 `json:"balance_usd"`
	AutoTopupEnabled         bool    `json:"auto_topup_enabled"`
	AutoTopupThresholdMicros int64   `json:"auto_topup_threshold_micros"`
	AutoTopupThresholdUSD    float64 `json:"auto_topup_threshold_usd"`
	AutoTopupAmountMicros    int64   `json:"auto_topup_amount_micros"`
	AutoTopupAmountUSD       float64 `json:"auto_topup_amount_usd"`
	WelcomeGranted           bool    `json:"welcome_granted"`
}

func toCreditsResponse(c *models.AICredits) creditsResponse {
	return creditsResponse{
		AccountID:                c.AccountID,
		BalanceMicros:            c.BalanceMicros,
		BalanceUSD:               c.BalanceUSD(),
		AutoTopupEnabled:         c.AutoTopupEnabled,
		AutoTopupThresholdMicros: c.AutoTopupThresholdMicros,
		AutoTopupThresholdUSD:    c.AutoTopupThresholdUSD(),
		AutoTopupAmountMicros:    c.AutoTopupAmountMicros,
		AutoTopupAmountUSD:       c.AutoTopupAmountUSD(),
		WelcomeGranted:           c.WelcomeGranted,
	}
}

func (c *aiCreditsInternalController) accountIDFromPath(w http.ResponseWriter, r *http.Request, requestID string) (uint64, bool) {
	params := mux.Vars(r)
	accountId, err := strconv.ParseUint(params["id"], 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("%s %s - invalid account ID parameter, id=%s", r.Method, r.URL.Path, params["id"]))
		utils.SendJSON(w, "invalid account id", false, http.StatusBadRequest, nil)
		return 0, false
	}
	if ctxAccountId, ok := utils.GetAccountID(r.Context()); ok && ctxAccountId != accountId {
		utils.SendJSON(w, "account mismatch: cannot access another account", false, http.StatusForbidden, nil)
		return 0, false
	}
	return accountId, true
}

func (c *aiCreditsInternalController) GetCredits(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	accountId, ok := c.accountIDFromPath(w, r, requestID)
	if !ok {
		return
	}
	credits, err := c.creditsService.Ensure(accountId)
	if err != nil {
		utils.SendJSON(w, err, false, err.Type, nil)
		return
	}
	utils.SendJSON(w, toCreditsResponse(credits), true, http.StatusOK, nil)
}

func (c *aiCreditsInternalController) Credit(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	accountId, ok := c.accountIDFromPath(w, r, requestID)
	if !ok {
		return
	}

	var body struct {
		AmountUSD             float64 `json:"amount_usd"`
		Kind                  string  `json:"kind"`
		StripePaymentIntentID string  `json:"stripe_payment_intent_id"`
		IdempotencyKey        string  `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.SendJSON(w, "invalid request body", false, http.StatusBadRequest, nil)
		return
	}
	credits, creditErr := c.creditsService.Credit(accountId, body.AmountUSD, body.Kind, body.StripePaymentIntentID, body.IdempotencyKey)
	if creditErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - credit failed, accountId=%d, error=%s", r.URL.Path, accountId, creditErr.Message))
		utils.SendJSON(w, creditErr, false, creditErr.Type, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - credit applied, accountId=%d, amountUsd=%.4f, kind=%s", r.URL.Path, accountId, body.AmountUSD, body.Kind))
	utils.SendJSON(w, toCreditsResponse(credits), true, http.StatusOK, nil)
}

func (c *aiCreditsInternalController) UpdateAutoTopup(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	accountId, ok := c.accountIDFromPath(w, r, requestID)
	if !ok {
		return
	}

	var body struct {
		Enabled      bool    `json:"enabled"`
		ThresholdUSD float64 `json:"threshold_usd"`
		AmountUSD    float64 `json:"amount_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.SendJSON(w, "invalid request body", false, http.StatusBadRequest, nil)
		return
	}
	credits, updateErr := c.creditsService.UpdateAutoTopup(accountId, body.Enabled, body.ThresholdUSD, body.AmountUSD)
	if updateErr != nil {
		utils.SendJSON(w, updateErr, false, updateErr.Type, nil)
		return
	}
	utils.SendJSON(w, toCreditsResponse(credits), true, http.StatusOK, nil)
}

func (c *aiCreditsInternalController) GetLedger(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	accountId, ok := c.accountIDFromPath(w, r, requestID)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := parseUintQuery(q.Get("limit"), 50)
	offset := parseUintQuery(q.Get("offset"), 0)

	entries, err := c.creditsService.GetLedger(accountId, limit, offset)
	if err != nil {
		utils.SendJSON(w, err, false, err.Type, nil)
		return
	}
	utils.SendJSON(w, map[string]any{"ledger": entries, "limit": limit, "offset": offset}, true, http.StatusOK, nil)
}
