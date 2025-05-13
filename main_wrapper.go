package main

import (
	"context"
	"log"
	"os"
)

// This file contains wrappers around the main functions to make them more testable.
// By separating the entry point from the business logic, we can more easily test
// the business logic without having to simulate command-line arguments.

// MainEntry is a testable wrapper for the main function
func MainEntry(args []string) int {
	// Save original args and restore them after execution
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	
	// Set args for parseFlags
	os.Args = args
	
	// Parse command line arguments
	config := parseFlags()
	
	// Run the cleanup
	summary, err := cleanupECR(config)
	if err != nil {
		log.Printf("Error cleaning up ECR repositories: %v", err)
		return 1
	}
	
	// Print summary
	log.Printf("ECR Cleanup Summary:")
	log.Printf("- Repositories processed: %d", summary.RepositoriesProcessed)
	log.Printf("- Images deleted: %d", summary.ImagesDeleted)
	if summary.SpaceFreed > 0 {
		log.Printf("- Space freed: %.2f MB", float64(summary.SpaceFreed)/1024/1024)
	}
	
	if config.DryRun {
		log.Printf("Note: This was a dry run. No images were actually deleted.")
	}
	
	return 0
}

// MainEntryWithClient is a testable version that accepts a client for testing
func MainEntryWithClient(args []string, client ECRClient) int {
	// Save original args and restore them after execution
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	
	// Set args for parseFlags
	os.Args = args
	
	// Parse command line arguments
	config := parseFlags()
	
	// Use our injected client
	ctx := context.Background()
	summary, err := CleanupWithClient(ctx, config, client)
	if err != nil {
		log.Printf("Error cleaning up ECR repositories: %v", err)
		return 1
	}
	
	// Print summary
	log.Printf("ECR Cleanup Summary:")
	log.Printf("- Repositories processed: %d", summary.RepositoriesProcessed)
	log.Printf("- Images deleted: %d", summary.ImagesDeleted)
	if summary.SpaceFreed > 0 {
		log.Printf("- Space freed: %.2f MB", float64(summary.SpaceFreed)/1024/1024)
	}
	
	if config.DryRun {
		log.Printf("Note: This was a dry run. No images were actually deleted.")
	}
	
	return 0
}

// main is the entry point for the application
func main() {
	exitCode := MainEntry(os.Args)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// CleanupWithClient is a testable version of cleanupECR that accepts a client
func CleanupWithClient(ctx context.Context, cfg Config, client ECRClient) (CleanupSummary, error) {
	summary := CleanupSummary{}
	
	// Get all repositories
	repos, err := getRepositories(ctx, client)
	if err != nil {
		return summary, err
	}
	
	summary.RepositoriesProcessed = len(repos)
	
	// Process each repository
	for _, repo := range repos {
		repoSummary, err := processRepository(ctx, client, *repo.RepositoryName, cfg)
		if err != nil {
			log.Printf("Error processing repository %s: %v", *repo.RepositoryName, err)
			continue
		}
		
		summary.ImagesDeleted += repoSummary.ImagesDeleted
		summary.SpaceFreed += repoSummary.SpaceFreed
	}
	
	return summary, nil
}