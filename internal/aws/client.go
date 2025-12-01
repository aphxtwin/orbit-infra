package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// Client wraps the AWS Lambda SDK client
// Provides methods to invoke Lambda functions for tenant provisioning
type Client struct {
	lambda *lambda.Client // AWS SDK Lambda client
	region string         // AWS region for Lambda invocations
}

// Config holds Lambda client configuration
type Config struct {
	Region string // AWS region (e.g., "us-east-1")
}

// NewClient creates a new Lambda client
// Loads AWS credentials from environment variables or IAM role
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	// Load AWS SDK configuration
	// Uses AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY from env
	// Or IAM role if running on EC2/ECS/Lambda
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		lambda: lambda.NewFromConfig(awsCfg),
		region: cfg.Region,
	}, nil
}

// invoke is the internal method that handles Lambda invocations
// payloadStruct: request payload (will be JSON-serialized)
// invocationType: RequestResponse (sync) or Event (async)
// Returns raw invocation result
func (c *Client) invoke(ctx context.Context, functionName string, payloadStruct interface{}, invocationType InvocationType) (*InvocationResult, error) {
	// Serialize payload to JSON
	payloadBytes, err := json.Marshal(payloadStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build Lambda invocation input
	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	}

	// Set invocation type based on parameter
	switch invocationType {
	case InvocationTypeRequestResponse:
		input.InvocationType = types.InvocationTypeRequestResponse // Synchronous
	case InvocationTypeEvent:
		input.InvocationType = types.InvocationTypeEvent // Asynchronous
	default:
		input.InvocationType = types.InvocationTypeRequestResponse
	}

	// Invoke the Lambda function
	result, err := c.lambda.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("Lambda invocation failed for %s: %w", functionName, err)
	}

	// Wrap the result
	invocationResult := &InvocationResult{
		RequestID:       "", // AWS SDK v2 doesn't expose RequestID in InvokeOutput
		StatusCode:      result.StatusCode,
		ExecutedVersion: aws.ToString(result.ExecutedVersion),
		LogResult:       aws.ToString(result.LogResult),
		Payload:         result.Payload,
	}

	// Check for function execution errors
	if result.FunctionError != nil {
		invocationResult.FunctionError = *result.FunctionError
	}

	return invocationResult, nil
}

// ProvisionTenant invokes the provisioning Lambda function
// Sends ProvisioningRequest, waits for and returns ProvisioningResponse
func (c *Client) ProvisionTenant(ctx context.Context, functionName string, req ProvisioningRequest) (*ProvisioningResponse, error) {
	// Invoke Lambda synchronously
	result, err := c.invoke(ctx, functionName, req, InvocationTypeRequestResponse)
	if err != nil {
		return nil, err
	}

	// Check for Lambda function errors
	if result.FunctionError != "" {
		return nil, fmt.Errorf("Lambda function error: %s", result.FunctionError)
	}

	// Check status code
	if result.StatusCode != 200 {
		return nil, fmt.Errorf("Lambda returned non-200 status: %d", result.StatusCode)
	}

	// Unmarshal response
	var response ProvisioningResponse
	if err := json.Unmarshal(result.Payload, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provisioning response: %w", err)
	}

	return &response, nil
}

// DeleteTenant invokes the deletion Lambda function
// Sends DeletionRequest, waits for and returns DeletionResponse
func (c *Client) DeleteTenant(ctx context.Context, functionName string, req DeletionRequest) (*DeletionResponse, error) {
	// Invoke Lambda synchronously
	result, err := c.invoke(ctx, functionName, req, InvocationTypeRequestResponse)
	if err != nil {
		return nil, err
	}

	// Check for function errors
	if result.FunctionError != "" {
		return nil, fmt.Errorf("Lambda function error: %s", result.FunctionError)
	}

	// Check status code
	if result.StatusCode != 200 {
		return nil, fmt.Errorf("Lambda returned non-200 status: %d", result.StatusCode)
	}

	// Unmarshal response
	var response DeletionResponse
	if err := json.Unmarshal(result.Payload, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deletion response: %w", err)
	}

	return &response, nil
}

// CheckStatus invokes the status check Lambda function
// Sends StatusCheckRequest, waits for and returns StatusCheckResponse
func (c *Client) CheckStatus(ctx context.Context, functionName string, req StatusCheckRequest) (*StatusCheckResponse, error) {
	// Invoke Lambda synchronously
	result, err := c.invoke(ctx, functionName, req, InvocationTypeRequestResponse)
	if err != nil {
		return nil, err
	}

	// Check for function errors
	if result.FunctionError != "" {
		return nil, fmt.Errorf("Lambda function error: %s", result.FunctionError)
	}

	// Check status code
	if result.StatusCode != 200 {
		return nil, fmt.Errorf("Lambda returned non-200 status: %d", result.StatusCode)
	}

	// Unmarshal response
	var response StatusCheckResponse
	if err := json.Unmarshal(result.Payload, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status response: %w", err)
	}

	return &response, nil
}

// InvokeAsync invokes a Lambda function asynchronously (fire and forget)
// Useful for triggering background tasks without waiting for completion
// Returns immediately after queueing the invocation
func (c *Client) InvokeAsync(ctx context.Context, functionName string, payload interface{}) error {
	_, err := c.invoke(ctx, functionName, payload, InvocationTypeEvent)
	return err
}

// InvokeSync invokes a Lambda function synchronously with a generic payload
// Useful for testing or invoking custom Lambda functions
// Returns the full invocation result including payload and metadata
func (c *Client) InvokeSync(ctx context.Context, functionName string, payload interface{}) (*InvocationResult, error) {
	return c.invoke(ctx, functionName, payload, InvocationTypeRequestResponse)
}