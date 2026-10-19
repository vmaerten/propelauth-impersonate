package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// fetchAPIKey reads PROPELAUTH_API_KEY from the JSON secret secretID of AWS Secrets Manager, with
// profile, or the active AWS profile when it is empty, in that profile's region.
func fetchAPIKey(ctx context.Context, secretID, profile string) (string, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return "", err
	}
	// The SDK's own fallbacks, to name the profile in the errors.
	active := cmp.Or(profile, os.Getenv("AWS_PROFILE"), os.Getenv("AWS_DEFAULT_PROFILE"), "default")
	if cfg.Region == "" {
		return "", fmt.Errorf("the AWS profile %s has no region: set it in ~/.aws/config, or AWS_REGION", active)
	}
	out, err := secretsmanager.NewFromConfig(cfg).GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretID),
	})
	if err != nil {
		return "", fmt.Errorf("reading %s (SSO session expired? aws sso login --profile %s): %w", secretID, active, err)
	}
	var secret struct {
		APIKey string `json:"PROPELAUTH_API_KEY"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &secret); err != nil {
		return "", fmt.Errorf("decoding %s: %w", secretID, err)
	}
	if secret.APIKey == "" {
		return "", errors.New(secretID + " has no PROPELAUTH_API_KEY")
	}
	return secret.APIKey, nil
}
