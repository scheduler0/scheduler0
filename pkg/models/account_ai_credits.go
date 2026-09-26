package models

import (
	"math"
	"time"
)

const AICreditMicrosPerUSD int64 = 1_000_000

const (
	AICreditKindWelcome   = "welcome"
	AICreditKindTopup     = "topup"
	AICreditKindAutoTopup = "auto_topup"
	AICreditKindConsume   = "consume"
	AICreditKindRefund    = "refund"
)

type AICredits struct {
	AccountID                uint64    `json:"accountId"`
	BalanceMicros            int64     `json:"balanceMicros"`
	AutoTopupEnabled         bool      `json:"autoTopupEnabled"`
	AutoTopupThresholdMicros int64     `json:"autoTopupThresholdMicros"`
	AutoTopupAmountMicros    int64     `json:"autoTopupAmountMicros"`
	WelcomeGranted           bool      `json:"welcomeGranted"`
	DateCreated              time.Time `json:"dateCreated"`
	DateModified             time.Time `json:"dateModified"`
}

func (c AICredits) BalanceUSD() float64 { return MicrosToUSD(c.BalanceMicros) }

func (c AICredits) AutoTopupThresholdUSD() float64 { return MicrosToUSD(c.AutoTopupThresholdMicros) }
func (c AICredits) AutoTopupAmountUSD() float64    { return MicrosToUSD(c.AutoTopupAmountMicros) }

type AICreditLedgerEntry struct {
	ID                    uint64    `json:"id"`
	AccountID             uint64    `json:"accountId"`
	AmountMicros          int64     `json:"amountMicros"`
	Kind                  string    `json:"kind"`
	BalanceAfterMicros    int64     `json:"balanceAfterMicros"`
	Provider              string    `json:"provider,omitempty"`
	Model                 string    `json:"model,omitempty"`
	PromptRequestID       uint64    `json:"promptRequestId,omitempty"`
	StripePaymentIntentID string    `json:"stripePaymentIntentId,omitempty"`
	IdempotencyKey        string    `json:"idempotencyKey"`
	DateCreated           time.Time `json:"dateCreated"`
}

func (e AICreditLedgerEntry) AmountUSD() float64 { return MicrosToUSD(e.AmountMicros) }

func USDToMicros(usd float64) int64 {
	return int64(math.Round(usd * float64(AICreditMicrosPerUSD)))
}

func USDToMicrosCeil(usd float64) int64 {
	if usd <= 0 {
		return 0
	}
	return int64(math.Ceil(usd * float64(AICreditMicrosPerUSD)))
}

func MicrosToUSD(micros int64) float64 {
	return float64(micros) / float64(AICreditMicrosPerUSD)
}
