package controllers

import (
	"fmt"
	"log"
	"net/http"
	"scheduler0-private/pkg/service/feature"
	"scheduler0-private/pkg/utils"
)

type FeatureController struct {
	featureService feature.FeatureService
	logger         *log.Logger
}

func NewFeatureController(logger *log.Logger, featureService feature.FeatureService) *FeatureController {
	return &FeatureController{
		featureService: featureService,
		logger:         logger,
	}
}

func (c *FeatureController) GetFeatures(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetFeatures entry", r.URL.Path))

	features, err := c.featureService.GetFeatures()
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetFeatures error: failed to get features, error=%s", r.URL.Path, err.Message))
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - GetFeatures success, status=200, count=%d", r.URL.Path, len(*features)))
	utils.SendJSON(w, features, true, http.StatusOK, nil)
}
