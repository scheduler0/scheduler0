package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/models"
	"scheduler0/utils"
	"strings"
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

func (httpExecutor *HTTPExecutionHandler) ExecuteHTTPJob(pendingJobs []*models.JobModel) error {
	urlJobCache := map[string][]*models.JobModel{}

	for _, pj := range pendingJobs {
		if pj != nil {
			if pJs, ok := urlJobCache[pj.CallbackUrl]; !ok {
				urlJobCache[pj.CallbackUrl] = []*models.JobModel{}
				urlJobCache[pj.CallbackUrl] = append(pJs, pj)
			} else {
				urlJobCache[pj.CallbackUrl] = append(pJs, pj)
			}
		}
	}

	var finalError error

	for rurl, uJc := range urlJobCache {

		batches := make([][]*models.JobModel, 0)

		if len(uJc) > 100 {
			temp := make([]*models.JobModel, 0)
			count := 0
			for count < len(uJc) {
				temp = append(temp, uJc[count])
				if len(temp) == 100 {
					batches = append(batches, temp)
					temp = make([]*models.JobModel, 0)
				}
				count += 1
			}
			if len(temp) > 0 {
				batches = append(batches, temp)
				temp = make([]*models.JobModel, 0)
			}
		} else {
			batches = append(batches, uJc)
		}

		configs := config.GetScheduler0Configurations(httpExecutor.logger)

		for _, batch := range batches {
			finalError = utils.RetryOnError(func() error {
				payload := make([]string, 0)

				for i := 0; i < len(batch); i += 1 {
					payload = append(payload, batch[i].Data)
				}

				strBuilder := new(strings.Builder)
				err := json.NewEncoder(strBuilder).Encode(payload)
				if err != nil {
					return err
				}
				toString := strBuilder.String()

				httpExecutor.logger.Println(fmt.Sprintf("Running Job Execution for Job CallbackURL = %v with payload len = %v",
					rurl, len(payload)))

				httpClient := http.Client{
					Timeout: time.Duration(configs.JobExecutionTimeout) * time.Second,
				}

				res, err := httpClient.Post(rurl, "application/json", strings.NewReader(toString))

				if res.StatusCode >= 200 || res.StatusCode <= 299 {
					return nil
				}

				return errors.New(fmt.Sprintf("subscriber failed to fully requests status code: %v", res.StatusCode))
			}, configs.JobExecutionRetryMax, configs.JobExecutionRetryDelay)
		}
	}

	return finalError
}
