package aws

import (
	"context"
	"encoding/json"
	"scheduler0/pkg/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/hashicorp/go-hclog"
)

// LambdaClientInterface defines the interface for Lambda client operations
// This allows for easier testing by injecting mock implementations
type LambdaClientInterface interface {
	Invoke(ctx context.Context, params *lambda.InvokeInput, optFns ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

type LambdaExecutionHandler struct {
	logger       hclog.Logger
	ctx          context.Context
	lambdaClient LambdaClientInterface
	configLoader func(ctx context.Context, region string, accessKey string, secretKey string) (aws.Config, error)
}

type LambdaExecutor interface {
	ExecuteLambdaJob(
		region string,
		functionArn string,
		accessKey string,
		secretKey string,
		pendingJob models.JobInvocationPayload,
		successCallback func(job models.Job),
		errorCallback func(job models.Job))
	ExecuteLambdaJobBatch(
		region string,
		functionArn string,
		accessKey string,
		secretKey string,
		pendingJobs models.AggregatedJobInvocationPayload,
		successCallback func(jobs []models.Job),
		errorCallback func(jobs []models.Job))
}

func NewLambdaExecutor(logger hclog.Logger, ctx context.Context) LambdaExecutor {
	return &LambdaExecutionHandler{
		logger: logger,
		ctx:    ctx,
		configLoader: func(ctx context.Context, region string, accessKey string, secretKey string) (aws.Config, error) {
			creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
			return config.LoadDefaultConfig(ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(creds),
			)
		},
	}
}

// NewLambdaExecutorWithClient creates a Lambda executor with a custom client (useful for testing)
func NewLambdaExecutorWithClient(logger hclog.Logger, ctx context.Context, lambdaClient LambdaClientInterface) LambdaExecutor {
	return &LambdaExecutionHandler{
		logger:       logger,
		ctx:          ctx,
		lambdaClient: lambdaClient,
	}
}

func (lambdaExecutor *LambdaExecutionHandler) ExecuteLambdaJob(
	region string,
	functionArn string,
	accessKey string,
	secretKey string,
	pendingJob models.JobInvocationPayload,
	successCallback func(job models.Job),
	errorCallback func(job models.Job)) {
	if region == "" {
		lambdaExecutor.logger.Error("missing AWS region for Lambda job", "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID)
		errorCallback(pendingJob.Job)
		return
	}
	if accessKey == "" || secretKey == "" {
		lambdaExecutor.logger.Error("missing AWS credentials for Lambda job", "region", region, "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID)
		errorCallback(pendingJob.Job)
		return
	}

	var lambdaClient LambdaClientInterface

	// If no client is injected, create one from config
	if lambdaExecutor.lambdaClient == nil {
		// Load AWS configuration
		var cfg aws.Config
		var err error
		if lambdaExecutor.configLoader != nil {
			cfg, err = lambdaExecutor.configLoader(lambdaExecutor.ctx, region, accessKey, secretKey)
		} else {
			creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
			cfg, err = config.LoadDefaultConfig(lambdaExecutor.ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(creds),
			)
		}
		if err != nil {
			lambdaExecutor.logger.Error("unable to load AWS config for Lambda job", "error", err, "region", region, "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
			errorCallback(pendingJob.Job)
			return
		}

		// Create Lambda client
		lambdaClient = lambda.NewFromConfig(cfg)
	} else {
		lambdaClient = lambdaExecutor.lambdaClient
	}

	// Convert jobs to JSON
	payload, err := json.Marshal(pendingJob)
	if err != nil {
		lambdaExecutor.logger.Error("failed to marshal jobs payload for Lambda job", "error", err, "region", region, "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}

	// Invoke Lambda function
	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionArn),
		Payload:      payload,
	}

	result, err := lambdaClient.Invoke(lambdaExecutor.ctx, input)
	if err != nil {
		lambdaExecutor.logger.Error("failed to invoke lambda function for Lambda job", "error", err, "region", region, "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}

	if result.FunctionError != nil {
		lambdaExecutor.logger.Error("lambda function returned error for Lambda job", "error", *result.FunctionError, "region", region, "functionArn", functionArn, "pendingJob.ID", pendingJob.ID, "pendingJob.ProjectID", pendingJob.ProjectID, "pendingJob.Spec", pendingJob.Spec, "pendingJob.Timezone", pendingJob.Timezone, "pendingJob.Data", pendingJob.Data, "pendingJob.Status", pendingJob.Status, "pendingJob.DateCreated", pendingJob.DateCreated)
		errorCallback(pendingJob.Job)
		return
	}

	successCallback(pendingJob.Job)
}

// ExecuteLambdaJobBatch invokes the Lambda once with an aggregated payload for a
// group of jobs sharing this executor and the same scheduled fire time. The
// whole batch succeeds or fails together, so the callbacks receive every job.
func (lambdaExecutor *LambdaExecutionHandler) ExecuteLambdaJobBatch(
	region string,
	functionArn string,
	accessKey string,
	secretKey string,
	pendingJobs models.AggregatedJobInvocationPayload,
	successCallback func(jobs []models.Job),
	errorCallback func(jobs []models.Job)) {
	jobs := pendingJobs.JobList()

	if region == "" {
		lambdaExecutor.logger.Error("missing AWS region for aggregated Lambda job", "functionArn", functionArn, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}
	if accessKey == "" || secretKey == "" {
		lambdaExecutor.logger.Error("missing AWS credentials for aggregated Lambda job", "region", region, "functionArn", functionArn, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	var lambdaClient LambdaClientInterface

	if lambdaExecutor.lambdaClient == nil {
		var cfg aws.Config
		var err error
		if lambdaExecutor.configLoader != nil {
			cfg, err = lambdaExecutor.configLoader(lambdaExecutor.ctx, region, accessKey, secretKey)
		} else {
			creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
			cfg, err = config.LoadDefaultConfig(lambdaExecutor.ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(creds),
			)
		}
		if err != nil {
			lambdaExecutor.logger.Error("unable to load AWS config for aggregated Lambda job", "error", err, "region", region, "functionArn", functionArn, "jobCount", len(jobs))
			errorCallback(jobs)
			return
		}

		lambdaClient = lambda.NewFromConfig(cfg)
	} else {
		lambdaClient = lambdaExecutor.lambdaClient
	}

	payload, err := json.Marshal(pendingJobs)
	if err != nil {
		lambdaExecutor.logger.Error("failed to marshal aggregated jobs payload for Lambda job", "error", err, "region", region, "functionArn", functionArn, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionArn),
		Payload:      payload,
	}

	result, err := lambdaClient.Invoke(lambdaExecutor.ctx, input)
	if err != nil {
		lambdaExecutor.logger.Error("failed to invoke lambda function for aggregated Lambda job", "error", err, "region", region, "functionArn", functionArn, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	if result.FunctionError != nil {
		lambdaExecutor.logger.Error("lambda function returned error for aggregated Lambda job", "error", *result.FunctionError, "region", region, "functionArn", functionArn, "jobCount", len(jobs))
		errorCallback(jobs)
		return
	}

	successCallback(jobs)
}
