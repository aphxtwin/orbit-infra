package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"provisioning/internal/aws"
	"provisioning/internal/config"
	"provisioning/internal/db"
	"provisioning/internal/handlers"
	"provisioning/internal/migrate"
	"provisioning/internal/orchestrator"
)

func main() {
	// Create context that listens for shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Load configuration from environment
	log.Println("Loading configuration...")
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded (environment: %s)", cfg.App.Environment)

	// Connect to database
	log.Println("Connecting to database...")
	database, err := db.NewDB(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("Database connection established")

	// Run migrations
	log.Println("Running database migrations...")
	migrationCfg := migrate.Config{
		DatabaseURL: fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
			cfg.Database.SSLMode,
		),
		MigrationsPath: "file://migrations",
	}

	if err := migrate.Run(migrationCfg); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	version, dirty, err := migrate.Version(migrationCfg)
	if err != nil {
		log.Printf("Warning: Could not get migration version: %v", err)
	} else {
		log.Printf("Migrations completed (version: %d, dirty: %v)", version, dirty)
	}

	// Initialize Lambda client
	log.Println("Initializing Lambda client...")
	lambdaClient, err := aws.NewClient(ctx, aws.Config{
		Region: cfg.AWS.Region,
	})
	if err != nil {
		log.Fatalf("Failed to create Lambda client: %v", err)
	}
	log.Printf("Lambda client initialized (region: %s)", cfg.AWS.Region)

	// Create orchestrator
	log.Println("Creating orchestrator...")
	orch := orchestrator.NewOrchestrator(orchestrator.Config{
		DB:                       database,
		LambdaClient:             lambdaClient,
		ProvisioningFunctionName: cfg.AWS.ProvisioningLambdaName,
		AppConfig:                cfg,
	})
	log.Printf("Orchestrator created (Lambda function: %s)", cfg.AWS.ProvisioningLambdaName)

	// Create tenant handler
	tenantHandler := handlers.NewTenantHandler(orch)

	// Setup HTTP server with health checks
	mux := http.NewServeMux()

	// Health check endpoint - checks if database is reachable
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "unhealthy: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "healthy")
	})

	// Ready check endpoint - confirms service is ready to accept traffic
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
	})

	// Tenant provisioning endpoint
	mux.HandleFunc("POST /tenants", tenantHandler.CreateTenant)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %d...", cfg.App.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Println("Service is running and ready to accept requests")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, starting graceful shutdown...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Service stopped gracefully")
}
