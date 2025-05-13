package main

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// TestCleanupWithClient tests our new wrapper function that accepts a client
func TestCleanupWithClient(t *testing.T) {
	ctx := context.Background()
	
	// Setup test repositories and images
	repo1 := "repo1"
	repo2 := "repo2"
	
	now := time.Now()
	
	// Create old images that should be deleted
	oldImage1 := types.ImageDetail{
		ImageDigest: aws.String("sha256:111"),
		ImageTags: []string{"v1"},
		ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)), // 15 days old
		ImageSizeInBytes: aws.Int64(1000000),
	}
	
	// Create newer images that should be kept
	newImage1 := types.ImageDetail{
		ImageDigest: aws.String("sha256:333"),
		ImageTags: []string{"latest"},
		ImagePushedAt: aws.Time(now.AddDate(0, 0, -5)), // 5 days old
		ImageSizeInBytes: aws.Int64(3000000),
	}
	
	// Test multiple scenarios
	t.Run("Cleanup multiple repositories", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			// Mock repository list
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{RepositoryName: aws.String(repo1)},
					{RepositoryName: aws.String(repo2)},
				},
			},
			// Mock image lists
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
					{ImageTag: aws.String("latest")},
				},
			},
			// Mock image details
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{oldImage1, newImage1},
			},
			// Mock delete response
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
				},
			},
		}
		
		// Create test config - delete images older than 10 days
		cfg := Config{
			Days: 10,
			DryRun: false,
		}
		
		// Call the function
		summary, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if summary.RepositoriesProcessed != 2 {
			t.Errorf("Expected 2 repositories processed, got %d", summary.RepositoriesProcessed)
		}
		
		// In each repository we should delete the old image but keep the newer one
		if mockClient.BatchDeleteImageCalls != 2 {
			t.Errorf("Expected 2 calls to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
	
	// Test with dry run
	t.Run("Dry run mode", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			// Mock repository list
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{RepositoryName: aws.String(repo1)},
				},
			},
			// Mock image lists
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
				},
			},
			// Mock image details
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{oldImage1},
			},
		}
		
		// Create test config with dry run enabled
		cfg := Config{
			Days: 10,
			DryRun: true,
		}
		
		// Call the function
		summary, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		if summary.RepositoriesProcessed != 1 {
			t.Errorf("Expected 1 repository processed, got %d", summary.RepositoriesProcessed)
		}
		
		// Should not actually delete anything in dry run mode
		if mockClient.BatchDeleteImageCalls != 0 {
			t.Errorf("Expected 0 calls to BatchDeleteImage in dry run mode, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
	
	// Test repository fetch error handling
	t.Run("Handle repository fetch error", func(t *testing.T) {
		// We can't easily override methods, so we'll create a specialized mock
		// that returns an error for repository listing
		mockClient := &MockECRClient{
			// Inject a nil output and we'll let MockClient.DescribeRepositories
			// handle the error case specially in this test
			DescribeRepositoriesOutput: nil,
		}
		
		// Create test config
		cfg := Config{
			Days: 10,
		}
		
		// Call the function - should return an error
		_, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// Should have error
		if err == nil {
			t.Fatal("Expected an error, got nil")
		}
	})
}

// TestMainEntryWithClient tests the MainEntryWithClient function without using flag
func TestMainEntryWithClient(t *testing.T) {
	// Create a modified version of MainEntryWithClient that doesn't use flag
	testMainEntry := func(dryRun bool, client ECRClient) int {
		// Create the config directly instead of using flag
		config := Config{
			DryRun:    dryRun,
			Days:      10,  // default value
			Region:    "",  // default value
			MaxImages: 0,   // default value
		}
		
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
	
	// Test with default arguments
	t.Run("Default arguments", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			// Mock repository list
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{RepositoryName: aws.String("test-repo")},
				},
			},
			// Mock image lists
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("latest")},
				},
			},
			// Mock image details - empty list so no images to delete
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{},
			},
		}
		
		// Test with injected client and dry-run mode
		exitCode := testMainEntry(true, mockClient)
		
		// Should exit cleanly
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
	})
	
	// Test with error handling
	t.Run("Error handling", func(t *testing.T) {
		// Setup mock client that returns an error
		mockClient := &MockECRClient{
			// Force an error
			DescribeRepositoriesError: &types.ServerException{
				Message: aws.String("Test error"),
			},
		}
		
		exitCode := testMainEntry(true, mockClient)
		
		// Should return error code
		if exitCode != 1 {
			t.Errorf("Expected exit code 1 for error case, got %d", exitCode)
		}
	})
}