package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"scheduler0/config"
	"scheduler0/models"
	"scheduler0/utils"
	"strconv"
	"time"
)

type HTTPExecutionHandler struct {
	logger     hclog.Logger
	ctx        context.Context
	config     config.Scheduler0Config
	dispatcher *utils.Dispatcher
}

//go:generate mockery --name HTTPExecutor --output ./ --inpackage
type HTTPExecutor interface {
	ExecuteHTTPJob(pendingJobs []models.Job, successCallback func(jobs []models.Job), errorCallback func(jobs []models.Job))
}

func NewHTTTPExecutor(logger hclog.Logger, ctx context.Context, config config.Scheduler0Config, dispatcher *utils.Dispatcher) HTTPExecutor {
	return &HTTPExecutionHandler{
		logger:     logger,
		ctx:        ctx,
		config:     config,
		dispatcher: dispatcher,
	}
}

func (httpExecutor *HTTPExecutionHandler) ExecuteHTTPJob(pendingJobs []models.Job, successCallback func(jobs []models.Job), errorCallback func(jobs []models.Job)) {
	urlJobCache := map[string][]models.Job{}

	for _, pj := range pendingJobs {
		if pJs, ok := urlJobCache[pj.CallbackUrl]; !ok {
			urlJobCache[pj.CallbackUrl] = []models.Job{}
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		} else {
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		}
	}

	for rurl, uJc := range urlJobCache {

		configs := config.NewScheduler0Config().GetConfigurations()
		batches := utils.BatchByBytes(uJc, int(configs.HTTPExecutorPayloadMaxSizeMb))

		for i, batch := range batches {
			func(url string, b []byte, chunkId int) {
				httpExecutor.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
					defer func() {
						close(errorChannel)
						close(successChannel)
					}()
					err := utils.RetryOnError(func() error {
						httpExecutor.logger.Info(fmt.Sprintf("running job execution for job callback url = %v", url))
						httpClient := http.Client{
							Timeout: time.Duration(configs.JobExecutionTimeout) * time.Second,
						}

						req, err := http.NewRequestWithContext(httpExecutor.ctx, http.MethodPost, url, bytes.NewReader(b))
						if err != nil {
							httpExecutor.logger.Error("failed to create request: ", "error", err.Error())
							errorCallback(httpExecutor.unwrapBatch(b))
							return err
						}
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("x-payload-chunk-id", strconv.FormatInt(int64(chunkId), 10))

						res, err := httpClient.Do(req)
						if err != nil {
							httpExecutor.logger.Error("request error: ", err.Error())
							errorCallback(httpExecutor.unwrapBatch(b))
							return err
						}

						if res.StatusCode >= 200 || res.StatusCode <= 299 {
							successCallback(httpExecutor.unwrapBatch(b))
							return nil
						}

						errorCallback(httpExecutor.unwrapBatch(b))
						return errors.New(fmt.Sprintf("subscriber failed to fully requests status code: %v", res.StatusCode))
					}, configs.JobExecutionRetryMax, configs.JobExecutionRetryDelay)
					if err != nil {
						httpExecutor.logger.Error("failed to execute jobs after retrying", "error", err)
					}
				})
			}(rurl, batch, i)
		}
	}
}

func (httpExecutor *HTTPExecutionHandler) unwrapBatch(data []byte) []models.Job {
	fj := []models.Job{}
	err := json.Unmarshal(data, &fj)
	if err != nil {
		httpExecutor.logger.Error("failed to marshal failed jobs: ", err.Error())
	}
	return fj
}
