package executors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/models"
	"scheduler0/utils"
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

		batches := make([][]models.JobModel, 0)

		if len(uJc) > 100 {
			temp := make([]models.JobModel, 0)
			count := 0
			for count < len(uJc) {
				temp = append(temp, uJc[count])
				if len(temp) == 100 {
					batches = append(batches, temp)
					temp = make([]models.JobModel, 0)
				}
				count += 1
			}
			if len(temp) > 0 {
				batches = append(batches, temp)
				temp = make([]models.JobModel, 0)
			}
		} else {
			batches = append(batches, uJc)
		}

		configs := config.GetScheduler0Configurations(httpExecutor.logger)

		for _, batch := range batches {
			go func(url string, b []models.JobModel) {
				utils.RetryOnError(func() error {
					by, err := json.Marshal(b)
					if err != nil {
						httpExecutor.logger.Println("failed to create request payload: ", err.Error())
						onFailure(b)
						return err
					}

					httpExecutor.logger.Println(fmt.Sprintf("Running Job Execution for Job CallbackURL = %v", url))

					httpClient := http.Client{
						Timeout: time.Duration(configs.JobExecutionTimeout) * time.Second,
					}

					req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(by))
					req.Header.Set("Content-Type", "application/json")
					if err != nil {
						httpExecutor.logger.Println("failed to create request: ", err.Error())
						onFailure(b)
						return err
					}

					res, err := httpClient.Do(req)
					if err != nil {
						httpExecutor.logger.Println("request error: ", err.Error())
						onFailure(b)
						return err
					}

					if res.StatusCode >= 200 || res.StatusCode <= 299 {
						onSuccess(b)
						return nil
					}

					onFailure(b)
					return errors.New(fmt.Sprintf("subscriber failed to fully requests status code: %v", res.StatusCode))
				}, configs.JobExecutionRetryMax, configs.JobExecutionRetryDelay)
			}(rurl, batch)
		}
	}

	return successJobs, failedJobs
}
