package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// Helper function to create a standard test mock client
func createMockClientForIntegrationTests() *MockECRClient {
	return &MockECRClient{
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
		// Mock image details
		DescribeImagesOutput: &ecr.DescribeImagesOutput{
			ImageDetails: []types.ImageDetail{},
		},
	}
}

// TestCleanupWithClientAdditionalCases adds more test cases for CleanupWithClient
func TestCleanupWithClientAdditionalCases(t *testing.T) {
	ctx := context.Background()
	
	// Test error cases not covered by existing tests
	t.Run("Image list error", func(t *testing.T) {
		// Create mock client with an error
		mockClient := createMockClientForIntegrationTests()
		mockClient.ListImagesError = &types.ServerException{Message: aws.String("List error")}
		
		// Run with mock client
		cfg := Config{
			Days: 10,
			DryRun: true,
		}
		
		summary, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// The function should return success, but have no processed images
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Should have processed the repository but with errors
		if summary.RepositoriesProcessed != 1 {
			t.Errorf("Expected 1 repository processed, got %d", summary.RepositoriesProcessed)
		}
		
		// Should not have deleted any images
		if summary.ImagesDeleted != 0 {
			t.Errorf("Expected 0 images deleted, got %d", summary.ImagesDeleted)
		}
	})
	
	t.Run("Describe images error", func(t *testing.T) {
		// Create mock client with an error in describe images
		mockClient := createMockClientForIntegrationTests()
		mockClient.DescribeImagesError = &types.ServerException{Message: aws.String("Describe error")}
		
		// Run with mock client
		cfg := Config{
			Days: 10,
			DryRun: true,
		}
		
		summary, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// The function should return success, but have no processed images
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Should have processed the repository but with errors
		if summary.RepositoriesProcessed != 1 {
			t.Errorf("Expected 1 repository processed, got %d", summary.RepositoriesProcessed)
		}
		
		// Should not have deleted any images
		if summary.ImagesDeleted != 0 {
			t.Errorf("Expected 0 images deleted, got %d", summary.ImagesDeleted)
		}
	})
	
	t.Run("Batch delete error", func(t *testing.T) {
		// Create a clock that returns an old image to delete
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
					{ImageTag: aws.String("old")},
				},
			},
			// Mock image details with an old image (20 days old)
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{
					{
						ImageDigest: aws.String("sha256:old"),
						ImageTags: []string{"old"},
						// Use a fixed time in the past (20 days ago)
						ImagePushedAt: aws.Time(time.Now().AddDate(0, 0, -20)),
						ImageSizeInBytes: aws.Int64(1000000),
					},
				},
			},
			// Add an error for batch delete
			BatchDeleteImageError: &types.ServerException{Message: aws.String("Delete error")},
		}
		
		// Run with mock client
		cfg := Config{
			Days: 1, // Delete images older than 1 day
			DryRun: false,
		}
		
		summary, err := CleanupWithClient(ctx, cfg, mockClient)
		
		// The function should return success, but have no processed images
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Should have processed the repository but with errors
		if summary.RepositoriesProcessed != 1 {
			t.Errorf("Expected 1 repository processed, got %d", summary.RepositoriesProcessed)
		}
		
		// Should not have deleted any images due to the error
		if summary.ImagesDeleted != 0 {
			t.Errorf("Expected 0 images deleted, got %d", summary.ImagesDeleted)
		}
	})
}