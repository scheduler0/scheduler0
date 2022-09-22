package executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0/server/models"
	"scheduler0/utils"
	"strings"
)

type HTTPExecutionHandler struct{}

type HTTPExecutor interface {
	ExecuteHTTPJob(pendingJobs []*models.JobModel) error
}

func NewHTTTPExecutor() *HTTPExecutionHandler {
	return &HTTPExecutionHandler{}
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

				utils.Info(fmt.Sprintf("Running Job Execution for Job CallbackURL = %v with payload len = %v",
					rurl, len(payload)))

				_, err = http.Post(rurl, "application/json", strings.NewReader(toString))

				return nil
			}, 20, 3)
		}
	}

	return finalError
}
