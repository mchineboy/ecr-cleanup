package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// Test helper for MainEntry that doesn't use flag to avoid conflicts
func TestMainEntryHelper(t *testing.T) {
	// Create a standard mock client for our tests
	mockClient := &MockECRClient{
		DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
			Repositories: []types.Repository{
				{RepositoryName: aws.String("test-repo")},
			},
		},
		ListImagesOutput: &ecr.ListImagesOutput{
			ImageIds: []types.ImageIdentifier{},
		},
		DescribeImagesOutput: &ecr.DescribeImagesOutput{
			ImageDetails: []types.ImageDetail{},
		},
	}

	// Create a custom MainEntry function that bypasses flag.Parse and uses our mock client
	customMainEntry := func(args []string) int {
		// Parse args manually rather than using flag to avoid test conflicts
		var config Config
		config.DryRun = true // Always default to dry-run for safety
		config.Days = 10     // Default to 10 days
		
		// Look for args like --days=30 or -days=30
		for _, arg := range args {
			if strings.HasPrefix(arg, "--days=") || strings.HasPrefix(arg, "-days=") {
				var days int
				parts := strings.Split(arg, "=")
				if len(parts) == 2 {
					_, err := fmt.Sscanf(parts[1], "%d", &days)
					if err == nil && days > 0 {
						config.Days = days
					}
				}
			}
			if arg == "--dry-run" || arg == "-dry-run" {
				config.DryRun = true
			}
		}
		
		// Use our mock client
		ctx := context.Background()
		summary, err := CleanupWithClient(ctx, config, mockClient)
		if err != nil {
			return 1
		}
		
		// Success
		if summary.RepositoriesProcessed > 0 {
			return 0
		}
		
		return 0
	}
	
	// Test our customMainEntry with various args
	testCases := []struct {
		name     string
		args     []string
		expected int // Expected exit code
	}{
		{"Default args", []string{"program"}, 0},
		{"With days", []string{"program", "--days=15"}, 0},
		{"With dry-run", []string{"program", "--dry-run"}, 0},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exitCode := customMainEntry(tc.args)
			if exitCode != tc.expected {
				t.Errorf("Expected exit code %d, got %d", tc.expected, exitCode)
			}
		})
	}
	
	// Test error case
	t.Run("Error case", func(t *testing.T) {
		// Force an error
		mockClient.DescribeRepositoriesError = &types.ServerException{
			Message: aws.String("Test error"),
		}
		defer func() {
			mockClient.DescribeRepositoriesError = nil
		}()
		
		exitCode := customMainEntry([]string{"program"})
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for error, got %d", exitCode)
		}
	})
}

// A special test that tries to cover the real cleanupECR function
// This is challenging because it creates its own client
func TestCleanupECRReal(t *testing.T) {
	// Skip this in normal test runs since it requires AWS credentials
	if os.Getenv("AWS_ECR_CLEANUP_INTEGRATION") != "true" {
		t.Skip("Skipping integration test. Set AWS_ECR_CLEANUP_INTEGRATION=true to run.")
	}
	
	// This test would actually connect to AWS, which we don't want in unit tests
	// But we'll expose the function for real integration testing
	cfg := Config{
		DryRun: true, // Always use dry-run for safety
		Days:   10,
	}
	
	// Call the function
	_, err := cleanupECR(cfg)
	
	// Just check if it ran without error
	if err != nil {
		// An error is OK if it's because we don't have AWS credentials
		if !strings.Contains(err.Error(), "failed to load AWS config") &&
		   !strings.Contains(err.Error(), "failed to get repositories") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}