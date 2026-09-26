package ai_credits

import (
	"net/http"
	"scheduler0/pkg/models"
	repo "scheduler0/pkg/repository/account_ai_credits"
	"scheduler0/pkg/utils"

	"github.com/hashicorp/go-hclog"
)

type LowCreditNotifier func(accountID uint64, balanceMicros int64, topupAmountMicros int64)

type AICreditsService interface {
	Ensure(accountID uint64) (*models.AICredits, *utils.GenericError)
	Get(accountID uint64) (*models.AICredits, *utils.GenericError)
	HasBalance(accountID uint64) (bool, *utils.GenericError)
	Credit(accountID uint64, amountUSD float64, kind string, stripePaymentIntentID string, idempotencyKey string) (*models.AICredits, *utils.GenericError)
	UpdateAutoTopup(accountID uint64, enabled bool, thresholdUSD float64, amountUSD float64) (*models.AICredits, *utils.GenericError)
	GetLedger(accountID uint64, limit uint64, offset uint64) ([]models.AICreditLedgerEntry, *utils.GenericError)
	ChargePlatformRun(accountID uint64, provider string, model string, costUSD float64)
	WelcomeCreditUSD() float64
	MarkupMultiplier() float64
	SetLowCreditNotifier(n LowCreditNotifier)
}

type aiCreditsService struct {
	repo       repo.AccountAICreditsRepo
	logger     hclog.Logger
	welcomeUSD float64
	markup     float64
	enabled    bool
	notifier   LowCreditNotifier
}

func NewAICreditsService(logger hclog.Logger, r repo.AccountAICreditsRepo, welcomeUSD float64, markup float64, enabled bool) AICreditsService {
	if markup <= 0 {
		markup = 1.20
	}
	return &aiCreditsService{
		repo:       r,
		logger:     logger.Named("ai-credits-service"),
		welcomeUSD: welcomeUSD,
		markup:     markup,
		enabled:    enabled,
	}
}

func (s *aiCreditsService) WelcomeCreditUSD() float64                { return s.welcomeUSD }
func (s *aiCreditsService) MarkupMultiplier() float64                { return s.markup }
func (s *aiCreditsService) SetLowCreditNotifier(n LowCreditNotifier) { s.notifier = n }

func (s *aiCreditsService) Ensure(accountID uint64) (*models.AICredits, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	welcomeMicros := int64(0)
	if s.enabled && s.welcomeUSD > 0 {
		welcomeMicros = models.USDToMicros(s.welcomeUSD)
	}
	return s.repo.Ensure(accountID, welcomeMicros)
}

func (s *aiCreditsService) Get(accountID uint64) (*models.AICredits, *utils.GenericError) {
	return s.Ensure(accountID)
}

func (s *aiCreditsService) HasBalance(accountID uint64) (bool, *utils.GenericError) {
	credits, err := s.Ensure(accountID)
	if err != nil {
		return false, err
	}
	return credits.BalanceMicros > 0, nil
}

func (s *aiCreditsService) Credit(accountID uint64, amountUSD float64, kind string, stripePaymentIntentID string, idempotencyKey string) (*models.AICredits, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	if amountUSD <= 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "amount must be greater than zero")
	}
	if kind == "" {
		kind = models.AICreditKindTopup
	}
	if _, ensureErr := s.Ensure(accountID); ensureErr != nil {
		return nil, ensureErr
	}
	return s.repo.ApplyDelta(accountID, repo.LedgerEntryInput{
		AmountMicros:          models.USDToMicros(amountUSD),
		Kind:                  kind,
		StripePaymentIntentID: stripePaymentIntentID,
		IdempotencyKey:        idempotencyKey,
	})
}

func (s *aiCreditsService) UpdateAutoTopup(accountID uint64, enabled bool, thresholdUSD float64, amountUSD float64) (*models.AICredits, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	threshold := int64(0)
	if thresholdUSD > 0 {
		threshold = models.USDToMicros(thresholdUSD)
	}
	amount := int64(0)
	if amountUSD > 0 {
		amount = models.USDToMicros(amountUSD)
	}
	return s.repo.UpdateAutoTopup(accountID, enabled, threshold, amount)
}

func (s *aiCreditsService) GetLedger(accountID uint64, limit uint64, offset uint64) ([]models.AICreditLedgerEntry, *utils.GenericError) {
	if accountID == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account ID is required")
	}
	return s.repo.GetLedger(accountID, limit, offset)
}

func (s *aiCreditsService) ChargePlatformRun(accountID uint64, provider string, model string, costUSD float64) {
	if accountID == 0 || costUSD <= 0 {
		return
	}
	billedMicros := models.USDToMicrosCeil(costUSD * s.markup)
	if billedMicros <= 0 {
		return
	}
	updated, err := s.repo.ApplyDelta(accountID, repo.LedgerEntryInput{
		AmountMicros: -billedMicros,
		Kind:         models.AICreditKindConsume,
		Provider:     provider,
		Model:        model,
	})
	if err != nil {
		s.logger.Error("ChargePlatformRun: failed to debit credits", "error", err, "accountID", accountID, "billedMicros", billedMicros)
		return
	}

	if s.notifier != nil && updated.AutoTopupEnabled && updated.BalanceMicros < updated.AutoTopupThresholdMicros {
		notifier := s.notifier
		balance := updated.BalanceMicros
		amount := updated.AutoTopupAmountMicros
		go notifier(accountID, balance, amount)
	}
}
