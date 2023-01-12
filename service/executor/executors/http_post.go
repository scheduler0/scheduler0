package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/models"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
	"time"
)

type HTTPExecutionHandler struct {
	logger *log.Logger
}

type HTTPExecutor interface {
	ExecuteHTTPJob(pendingJobs []*models.JobModel) error
}

func NewHTTTPExecutor(logger *log.Logger) *HTTPExecutionHandler {
	return &HTTPExecutionHandler{
		logger: logger,
	}
}

func (httpExecutor *HTTPExecutionHandler) ExecuteHTTPJob(ctx context.Context, pendingJobs []models.JobModel, onSuccess func(jobs []models.JobModel), onFailure func(jobs []models.JobModel)) ([]models.JobModel, []models.JobModel) {
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

		configs := config.GetConfigurations(httpExecutor.logger)
		batches := batcher.BatchByBytes(uJc, int(configs.HTTPExecutorPayloadMaxSizeMb))

		for i, batch := range batches {
			go func(url string, b []byte, chunkId int) {
				utils.RetryOnError(func() error {
					httpExecutor.logger.Println(fmt.Sprintf("running job execution for job callback url = %v", url))
					httpClient := http.Client{
						Timeout: time.Duration(configs.JobExecutionTimeout) * time.Second,
					}

					req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
					req.Header.Set("Content-Type", "application/json")
					//req.Header.Set("x-payload-chunk-id", string(rune(chunkId)))
					if err != nil {
						httpExecutor.logger.Println("failed to create request: ", err.Error())
						onFailure(httpExecutor.unwrapBatch(b))
						return err
					}

					res, err := httpClient.Do(req)
					if err != nil {
						httpExecutor.logger.Println("request error: ", err.Error())
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
			}(rurl, batch, i)
		}
	}

	return successJobs, failedJobs
}

func (httpExecutor *HTTPExecutionHandler) unwrapBatch(data []byte) []models.JobModel {
	fj := []models.JobModel{}
	err := json.Unmarshal(data, &fj)
	if err != nil {
		httpExecutor.logger.Fatalln("failed to marshal failed jobs: ", err.Error())
	}
	return fj
}
