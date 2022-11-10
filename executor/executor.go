package executor

import (
	"fmt"
	"github.com/spf13/afero"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/models"
	"strings"
	"time"
)

type Executor interface {
	ExecuteHTTP() error
}

type Service struct {
	pendingJob           []*models.JobModel
	httpExecutionHandler *HTTPExecutionHandler
	onSuccess            func(pj []*models.JobModel)
	onFailure            func(pj []*models.JobModel, err error)
}

func NewService(logger *log.Logger, pj []*models.JobModel, onSuccess func(pj []*models.JobModel), onFailure func(pj []*models.JobModel, err error)) *Service {
	return &Service{
		pendingJob:           pj,
		httpExecutionHandler: NewHTTTPExecutor(logger),
		onSuccess:            onSuccess,
		onFailure:            onFailure,
	}
}

func (executorService *Service) ExecuteHTTP() {
	go func(pjs []*models.JobModel) {
		err := executorService.httpExecutionHandler.ExecuteHTTPJob(pjs)
		if err != nil {
			executorService.onFailure(pjs, err)
			return
		}
		executorService.onSuccess(pjs)
		for _, pendingJob := range pjs {
			go executorService.WriteJobExecutionLog(*pendingJob)
		}
	}(executorService.pendingJob)
}

func (executorService *Service) WriteJobExecutionLog(job models.JobModel) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	logFilePath := fmt.Sprintf("%v/%v/%v.txt", dir, constants.ExecutionLogsDir, job.ID)

	fs := afero.NewOsFs()

	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		return
	}

	if !exists {
		err := fs.Mkdir(dirPath, os.ModePerm)
		if err != nil {
			return
		}
	}

	logs := []string{}
	lines := []string{}

	fileData, err := afero.ReadFile(fs, logFilePath)
	if err == nil {
		dataString := string(fileData)
		lines = strings.Split(dataString, "\n")
	}

	logStr := fmt.Sprintf("execute %v, %v", job.ID, time.Now().UTC())
	logs = append(logs, logStr)
	logs = append(logs, lines...)

	str := strings.Join(logs, "\n")
	sliceByte := []byte(str)

	writeErr := afero.WriteFile(fs, logFilePath, sliceByte, os.ModePerm)
	if writeErr != nil {
		log.Fatalln("Binary Write Error::", writeErr)
	}
}
