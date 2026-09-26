package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/utils"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/afero"
)

type Scheduler0Secrets interface {
	GetSecrets() *scheduler0Secrets
	GetSecretsSkipCache() *scheduler0Secrets
	SaveSecrets(credentialsInput *scheduler0Secrets) *scheduler0Secrets
}

type scheduler0Secrets struct {
	SecretKey    string `json:"secretKey" yaml:"SecretKey"`
	AuthUsername string `json:"authUsername" yaml:"AuthUsername"`
	AuthPassword string `json:"authPassword" yaml:"AuthPassword"`
	BaseURL      string `json:"baseURL" yaml:"BaseURL"`
}

func NewScheduler0Secrets() *scheduler0Secrets {
	return &scheduler0Secrets{}
}

var cachedSecrets *scheduler0Secrets

// GetSecrets retrieves scheduler0 credentials in the following priority order:
//  1. Secrets file on disk (constants.SecretsFileName)
//  2. AWS SSM Parameter Store   – when SCHEDULER0_SECRETS_SOURCE=ssm
//     Requires SCHEDULER0_SSM_SECRETS_PATH (e.g. /scheduler0/production/)
//  3. AWS Secrets Manager       – when SCHEDULER0_SECRETS_SOURCE=aws_secrets
//     Requires SCHEDULER0_AWS_SECRETS_ID (name or ARN of the secret)
//  4. Environment variables     (last resort)
//
// Both AWS sources honour SCHEDULER0_AWS_REGION for the client region.
func (_ *scheduler0Secrets) GetSecrets() *scheduler0Secrets {
	if cachedSecrets != nil {
		return cachedSecrets
	}

	cachedSecrets = loadSecrets()
	return cachedSecrets
}

// GetSecretsSkipCache reloads secrets directly from the configured source,
// bypassing the in-memory cache, and refreshes the cache with the freshly
// loaded values.
//
// Use this when the process has already cached secrets but the underlying
// source may have changed since — for example during secret rotation, where an
// operator updates the SecretKey in the secrets source and the running server
// must pick up the new key without a restart.
func (_ *scheduler0Secrets) GetSecretsSkipCache() *scheduler0Secrets {
	cachedSecrets = loadSecrets()
	return cachedSecrets
}

// loadSecrets reads secrets from the configured source (file, AWS SSM, AWS
// Secrets Manager, or environment variables) without touching the cache.
func loadSecrets() *scheduler0Secrets {
	binPath := utils.GetBinPath()

	secretsPath := binPath + "/" + constants.SecretsFileName
	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, secretsPath)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	secrets := scheduler0Secrets{}

	if !os.IsNotExist(err) {
		// Secrets file found – use it exclusively.
		if jsonErr := json.Unmarshal(data, &secrets); jsonErr != nil {
			panic(jsonErr)
		}
		log.Printf("[secrets] loaded from file %s (fields: %s)", secretsPath, loadedSecretFields(&secrets))
		return &secrets
	}

	log.Printf("[secrets] no secrets file at %s", secretsPath)

	// No secrets file – try an AWS source if configured.
	source := os.Getenv("SCHEDULER0_SECRETS_SOURCE")
	region := os.Getenv("SCHEDULER0_AWS_REGION")

	switch source {
	case "ssm":
		ssmPath := os.Getenv("SCHEDULER0_SSM_SECRETS_PATH")
		if ssmPath != "" {
			log.Printf("[secrets] loading from AWS SSM path=%s region=%s", ssmPath, secretsRegionLabel(region))
			secrets = getSecretsFromSSM(ssmPath, region)
			return &secrets
		}
		log.Printf("[secrets] SCHEDULER0_SECRETS_SOURCE=ssm but SCHEDULER0_SSM_SECRETS_PATH is empty; falling back to environment variables")
	case "aws_secrets":
		secretID := os.Getenv("SCHEDULER0_AWS_SECRETS_ID")
		if secretID != "" {
			log.Printf("[secrets] loading from AWS Secrets Manager secret_id=%s region=%s", secretID, secretsRegionLabel(region))
			secrets = getSecretsFromAWSSecretsManager(secretID, region)
			return &secrets
		}
		log.Printf("[secrets] SCHEDULER0_SECRETS_SOURCE=aws_secrets but SCHEDULER0_AWS_SECRETS_ID is empty; falling back to environment variables")
	default:
		if source != "" {
			log.Printf("[secrets] unknown SCHEDULER0_SECRETS_SOURCE=%q; falling back to environment variables", source)
		}
	}

	// Fall back to plain environment variables.
	log.Printf("[secrets] loading from environment variables")
	secrets = getSecretsFromEnv()
	log.Printf("[secrets] loaded from environment variables (fields: %s)", loadedSecretFields(&secrets))
	return &secrets
}

// SaveSecrets saves the secrets into a .scheduler0 file
func (_ *scheduler0Secrets) SaveSecrets(credentialsInput *scheduler0Secrets) *scheduler0Secrets {
	binPath := utils.GetBinPath()

	fs := afero.NewOsFs()
	data, err := json.Marshal(credentialsInput)
	if err != nil {
		panic(err)
	}

	err = afero.WriteFile(fs, binPath+"/"+constants.SecretsFileName, data, os.ModePerm)
	if err != nil {
		panic(err)
	}

	cachedSecrets = credentialsInput

	return cachedSecrets
}

// getSecretsFromSSM fetches individual parameters stored under pathPrefix.
// Each secret field is expected as a separate parameter, e.g.:
//
//	{pathPrefix}SecretKey
//	{pathPrefix}AuthUsername
//	{pathPrefix}AuthPassword
//	{pathPrefix}BaseURL
func getSecretsFromSSM(pathPrefix string, region string) scheduler0Secrets {
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config for SSM: %v", err))
	}

	client := ssm.NewFromConfig(cfg)

	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}

	withDecryption := true
	resp, err := client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:           aws.String(pathPrefix),
		WithDecryption: &withDecryption,
		Recursive:      aws.Bool(false),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to get SSM parameters from path %s: %v", pathPrefix, err))
	}

	secrets := scheduler0Secrets{}
	for _, param := range resp.Parameters {
		if param.Name == nil || param.Value == nil {
			continue
		}
		key := strings.TrimPrefix(*param.Name, pathPrefix)
		val := *param.Value
		switch key {
		case "SCHEDULER0_SECRET_KEY":
			secrets.SecretKey = val
		case "SCHEDULER0_AUTH_USERNAME":
			secrets.AuthUsername = val
		case "SCHEDULER0_AUTH_PASSWORD":
			secrets.AuthPassword = val
		case "BaseURL":
			secrets.BaseURL = val
		}
	}

	log.Printf("[secrets] loaded from AWS SSM path=%s (fields: %s)", pathPrefix, loadedSecretFields(&secrets))

	return secrets
}

// getSecretsFromAWSSecretsManager fetches a single secret by ID and unmarshals
// its JSON string value into scheduler0Secrets.  The secret must be stored as a
// JSON object with keys matching the scheduler0Secrets JSON field names, e.g.:
//
//	{"secretKey":"…","authUsername":"…","authPassword":"…","baseURL":"…"}
func getSecretsFromAWSSecretsManager(secretID string, region string) scheduler0Secrets {
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config for Secrets Manager: %v", err))
	}

	client := secretsmanager.NewFromConfig(cfg)

	resp, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	})
	if err != nil {
		panic(fmt.Sprintf("failed to get secret %q from AWS Secrets Manager: %v", secretID, err))
	}

	secrets := scheduler0Secrets{}
	if resp.SecretString != nil {
		if jsonErr := json.Unmarshal([]byte(*resp.SecretString), &secrets); jsonErr != nil {
			panic(fmt.Sprintf("failed to parse secret JSON from AWS Secrets Manager: %v", jsonErr))
		}
	}

	log.Printf("[secrets] loaded from AWS Secrets Manager secret_id=%s (fields: %s)", secretID, loadedSecretFields(&secrets))

	return secrets
}

func secretsRegionLabel(region string) string {
	if region == "" {
		return "(default AWS region)"
	}
	return region
}

func loadedSecretFields(secrets *scheduler0Secrets) string {
	var fields []string
	if secrets.SecretKey != "" {
		fields = append(fields, "SecretKey")
	}
	if secrets.AuthUsername != "" {
		fields = append(fields, "AuthUsername")
	}
	if secrets.AuthPassword != "" {
		fields = append(fields, "AuthPassword")
	}
	if secrets.BaseURL != "" {
		fields = append(fields, "BaseURL")
	}
	if len(fields) == 0 {
		return "none"
	}
	return strings.Join(fields, ", ")
}

func getSecretsFromEnv() scheduler0Secrets {
	secrets := scheduler0Secrets{}

	if val, ok := os.LookupEnv("SCHEDULER0_SECRET_KEY"); ok {
		secrets.SecretKey = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_PASSWORD"); ok {
		secrets.AuthPassword = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_AUTH_USERNAME"); ok {
		secrets.AuthUsername = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_BASE_URL"); ok {
		secrets.BaseURL = val
	}

	return secrets
}
