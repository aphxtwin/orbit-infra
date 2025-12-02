package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"provisioning/internal/aws"
	"provisioning/internal/config"
)

// TestPayload is a simple test payload to send to Lambda
type TestPayload struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// TestResponse is the expected response from a simple test Lambda
type TestResponse struct {
	StatusCode int                    `json:"statusCode"`
	Message    string                 `json:"message"`
	Received   map[string]interface{} `json:"received"`
}

func main() {
	ctx := context.Background()

	log.Println("ctx:", ctx)

	// Load configuration from .env file
	log.Println("🔧 Loading configuration...")
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("❌ Failed to load configuration: %v", err)
	}

	// Display AWS configuration (without showing full credentials)
	log.Printf("📍 AWS Region: %s", cfg.AWS.Region)
	accessKeyPreview := "****"
	if len(os.Getenv("AWS_ACCESS_KEY_ID")) > 4 {
		accessKeyPreview = os.Getenv("AWS_ACCESS_KEY_ID")[:4] + "****"
	}
	log.Printf("🔑 AWS Access Key: %s", accessKeyPreview)
	log.Println()

	// Initialize Lambda client
	log.Println("🚀 Initializing Lambda client...")
	lambdaClient, err := aws.NewClient(ctx, aws.Config{
		Region: cfg.AWS.Region,
	})
	if err != nil {
		log.Fatalf("❌ Failed to create Lambda client: %v", err)
	}
	log.Println("✅ Lambda client created successfully")
	log.Println()

	// Get Lambda function name from command line or use default
	functionName := "test-hello-world"
	if len(os.Args) > 1 {
		functionName = os.Args[1]
	}

	// Create test payload
	payload := TestPayload{
		Name:    "Orbit Test",
		Message: "Testing Lambda connection from Go application",
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📤 Invoking Lambda function: %s", functionName)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("Payload:\n%s\n", string(payloadJSON))

	// Invoke Lambda synchronously
	result, err := lambdaClient.InvokeSync(ctx, functionName, payload)
	if err != nil {
		log.Fatalf("❌ Lambda invocation failed: %v", err)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("📥 Lambda Response")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("Status Code: %d", result.StatusCode)

	if result.FunctionError != "" {
		log.Printf("⚠️  Function Error: %s", result.FunctionError)
	}

	if result.ExecutedVersion != "" {
		log.Printf("Executed Version: %s", result.ExecutedVersion)
	}

	// Display the raw response
	if len(result.Payload) > 0 {
		log.Println("\nRaw Response:")

		// Try to pretty-print JSON
		var prettyJSON interface{}
		if err := json.Unmarshal(result.Payload, &prettyJSON); err == nil {
			prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
			log.Printf("%s", string(prettyBytes))
		} else {
			log.Printf("%s", string(result.Payload))
		}

		// Try to parse as TestResponse
		log.Println("\nParsed Response:")
		var testResp TestResponse
		if err := json.Unmarshal(result.Payload, &testResp); err != nil {
			log.Printf("(Response doesn't match TestResponse structure - this is OK)")
		} else {
			log.Printf("  Status Code: %d", testResp.StatusCode)
			log.Printf("  Message: %s", testResp.Message)
			if testResp.Received != nil {
				receivedJSON, _ := json.MarshalIndent(testResp.Received, "  ", "  ")
				log.Printf("  Received: %s", string(receivedJSON))
			}
		}
	}

	log.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🎉 Lambda connection test completed successfully!")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
