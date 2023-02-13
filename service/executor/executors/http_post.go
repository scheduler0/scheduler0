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
	"scheduler0/utils/batcher"
	"scheduler0/utils/workers"
	"strconv"
	"time"
)

type HTTPExecutionHandler struct {
	logger hclog.Logger
}

type HTTPExecutor interface {
	ExecuteHTTPJob(pendingJobs []*models.JobModel) error
}

func NewHTTTPExecutor(logger hclog.Logger) *HTTPExecutionHandler {
	return &HTTPExecutionHandler{
		logger: logger,
	}
}

func (httpExecutor *HTTPExecutionHandler) ExecuteHTTPJob(ctx context.Context, dispatcher *workers.Dispatcher, pendingJobs []models.JobModel, onSuccess func(jobs []models.JobModel), onFailure func(jobs []models.JobModel)) ([]models.JobModel, []models.JobModel) {
	urlJobCache := map[string][]models.JobModel{}

	for _, pj := range pendingJobs {
		if pJs, ok := urlJobCache[pj.CallbackUrl]; !ok {
			urlJobCache[pj.CallbackUrl] = []models.JobModel{}
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		} else {
			urlJobCache[pj.CallbackUrl] = append(pJs, pj)
		}
	}

	failedJobs := []models.JobModel{}
	successJobs := []models.JobModel{}

	for rurl, uJc := range urlJobCache {

		configs := config.GetConfigurations()
		batches := batcher.BatchByBytes(uJc, int(configs.HTTPExecutorPayloadMaxSizeMb))

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
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("x-payload-chunk-id", strconv.FormatInt(int64(chunkId), 10))
						if err != nil {
							httpExecutor.logger.Error("failed to create request: ", err.Error())
							onFailure(httpExecutor.unwrapBatch(b))
							return err
						}

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

func (httpExecutor *HTTPExecutionHandler) unwrapBatch(data []byte) []models.JobModel {
	fj := []models.JobModel{}
	err := json.Unmarshal(data, &fj)
	if err != nil {
		httpExecutor.logger.Error("failed to marshal failed jobs: ", err.Error())
	}
	return fj
}
