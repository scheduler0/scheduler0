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
	logger hclog.Logger
	config config.Scheduler0Config
}

type HTTPExecutor interface {
	ExecuteHTTPJob(pendingJobs []*models.Job) error
}

func NewHTTTPExecutor(logger hclog.Logger) *HTTPExecutionHandler {
	return &HTTPExecutionHandler{
		logger: logger,
	}
}

func (httpExecutor *HTTPExecutionHandler) ExecuteHTTPJob(ctx context.Context, dispatcher *utils.Dispatcher, pendingJobs []models.Job, onSuccess func(jobs []models.Job), onFailure func(jobs []models.Job)) ([]models.Job, []models.Job) {
	urlJobCache := map[string][]models.Job{}

	for _, pj := range pendingJobs {
		if pJs, ok := urlJobCache[pj.CallbackUrl]; !ok {
			urlJobCache[pj.CallbackUrl] = []models.Job{}
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		} else {
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		}
	}

	failedJobs := []models.Job{}
	successJobs := []models.Job{}

	for rurl, uJc := range urlJobCache {

		configs := config.NewScheduler0Config().GetConfigurations()
		batches := utils.BatchByBytes(uJc, int(configs.HTTPExecutorPayloadMaxSizeMb))

		for i, batch := range batches {
			func(url string, b []byte, chunkId int) {
				dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
					defer func() {
						close(errorChannel)
						close(successChannel)
					}()

					utils.RetryOnError(func() error {
						httpExecutor.logger.Info(fmt.Sprintf("running job execution for job callback url = %v", url))
						httpClient := http.Client{
							Timeout: time.Duration(configs.JobExecutionTimeout) * time.Second,
						}

						req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
						if err != nil {
							httpExecutor.logger.Error("failed to create request: ", "error", err.Error())
							onFailure(httpExecutor.unwrapBatch(b))
							return err
						}
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("x-payload-chunk-id", strconv.FormatInt(int64(chunkId), 10))

						res, err := httpClient.Do(req)
						if err != nil {
							httpExecutor.logger.Error("request error: ", err.Error())
							onFailure(httpExecutor.unwrapBatch(b))
							return err
						}

						if res.StatusCode >= 200 || res.StatusCode <= 299 {
							onSuccess(httpExecutor.unwrapBatch(b))
							return nil
						}

						onFailure(httpExecutor.unwrapBatch(b))
						return errors.New(fmt.Sprintf("subscriber failed to fully requests status code: %v", res.StatusCode))
					}, configs.JobExecutionRetryMax, configs.JobExecutionRetryDelay)
				})
			}(rurl, batch, i)
		}
	}

	return successJobs, failedJobs
}

func (httpExecutor *HTTPExecutionHandler) unwrapBatch(data []byte) []models.Job {
	fj := []models.Job{}
	err := json.Unmarshal(data, &fj)
	if err != nil {
		httpExecutor.logger.Error("failed to marshal failed jobs: ", err.Error())
	}
	return fj
}
