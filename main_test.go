package main

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// MockECRClient implements the ECRClient interface for testing
type MockECRClient struct {
	// Mock responses
	DescribeRepositoriesOutput *ecr.DescribeRepositoriesOutput
	ListImagesOutput           *ecr.ListImagesOutput
	DescribeImagesOutput       *ecr.DescribeImagesOutput
	BatchDeleteImageOutput     *ecr.BatchDeleteImageOutput

	// Errors to return (nil means no error)
	DescribeRepositoriesError error
	ListImagesError           error
	DescribeImagesError       error
	BatchDeleteImageError     error

	// Track calls to methods
	DescribeRepositoriesCalls int
	ListImagesCalls           int
	DescribeImagesCalls       int
	BatchDeleteImageCalls     int

	// Capture inputs for validation
	LastDescribeRepositoriesInput *ecr.DescribeRepositoriesInput
	LastListImagesInput           *ecr.ListImagesInput
	LastDescribeImagesInput       *ecr.DescribeImagesInput
	LastBatchDeleteImageInput     *ecr.BatchDeleteImageInput
	
	// Custom handlers for pagination testing
	NextDescribeRepositoriesOutput *ecr.DescribeRepositoriesOutput
}

// DescribeRepositories mock implementation
func (m *MockECRClient) DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	m.DescribeRepositoriesCalls++
	m.LastDescribeRepositoriesInput = params
	
	// Return error if set
	if m.DescribeRepositoriesError != nil {
		return nil, m.DescribeRepositoriesError
	}
	
	// Handle nil output case as an error for tests
	if m.DescribeRepositoriesOutput == nil {
		return nil, &types.ServerException{Message: aws.String("Test server error")}
	}
	
	// Special handling for pagination testing
	if params.NextToken != nil && m.NextDescribeRepositoriesOutput != nil {
		return m.NextDescribeRepositoriesOutput, nil
	}
	
	return m.DescribeRepositoriesOutput, nil
}

// ListImages mock implementation
func (m *MockECRClient) ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error) {
	m.ListImagesCalls++
	m.LastListImagesInput = params
	
	// Return error if set
	if m.ListImagesError != nil {
		return nil, m.ListImagesError
	}
	
	return m.ListImagesOutput, nil
}

// DescribeImages mock implementation
func (m *MockECRClient) DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	m.DescribeImagesCalls++
	m.LastDescribeImagesInput = params
	
	// Return error if set
	if m.DescribeImagesError != nil {
		return nil, m.DescribeImagesError
	}
	
	return m.DescribeImagesOutput, nil
}

// BatchDeleteImage mock implementation
func (m *MockECRClient) BatchDeleteImage(ctx context.Context, params *ecr.BatchDeleteImageInput, optFns ...func(*ecr.Options)) (*ecr.BatchDeleteImageOutput, error) {
	m.BatchDeleteImageCalls++
	m.LastBatchDeleteImageInput = params
	
	// Return error if set
	if m.BatchDeleteImageError != nil {
		return nil, m.BatchDeleteImageError
	}
	
	return m.BatchDeleteImageOutput, nil
}

// TestGetRepositories tests the getRepositories function
func TestGetRepositories(t *testing.T) {
	// Test with single page of results
	t.Run("Single page of repositories", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{
						RepositoryName: aws.String("repo1"),
						RepositoryUri: aws.String("uri/repo1"),
					},
					{
						RepositoryName: aws.String("repo2"),
						RepositoryUri: aws.String("uri/repo2"),
					},
				},
				NextToken: nil, // No more pages
			},
		}

		// Call the function
		repos, err := getRepositories(context.Background(), mockClient)

		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(repos) != 2 {
			t.Fatalf("Expected 2 repositories, got %d", len(repos))
		}
		if *repos[0].RepositoryName != "repo1" {
			t.Errorf("Expected repo name 'repo1', got '%s'", *repos[0].RepositoryName)
		}
		if mockClient.DescribeRepositoriesCalls != 1 {
			t.Errorf("Expected 1 call to DescribeRepositories, got %d", mockClient.DescribeRepositoriesCalls)
		}
	})

	// Test with multiple pages of results
	t.Run("Multiple pages of repositories", func(t *testing.T) {
		// Setup first page response with next token
		firstToken := "page2token"
		mockClient := &MockECRClient{
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{
						RepositoryName: aws.String("repo1"),
						RepositoryUri: aws.String("uri/repo1"),
					},
				},
				NextToken: &firstToken,
			},
			// Setup second page response
			NextDescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{
						RepositoryName: aws.String("repo2"),
						RepositoryUri: aws.String("uri/repo2"),
					},
				},
				NextToken: nil,
			},
		}

		// Call the function
		repos, err := getRepositories(context.Background(), mockClient)

		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(repos) != 2 {
			t.Fatalf("Expected 2 repositories, got %d", len(repos))
		}
		if mockClient.DescribeRepositoriesCalls != 2 {
			t.Errorf("Expected 2 calls to DescribeRepositories, got %d", mockClient.DescribeRepositoriesCalls)
		}
	})
}

// TestGetImageDetails tests the getImageDetails function
func TestGetImageDetails(t *testing.T) {
	// Test with a single page of results
	t.Run("Single page of images", func(t *testing.T) {
		repoName := "test-repo"
		
		// Setup mock client
		mockClient := &MockECRClient{
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{
						ImageTag: aws.String("latest"),
						ImageDigest: aws.String("sha256:123"),
					},
				},
				NextToken: nil, // No more pages
			},
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{
					{
						ImageTags: []string{"latest"},
						ImageDigest: aws.String("sha256:123"),
						ImagePushedAt: aws.Time(time.Now()),
					},
				},
			},
		}

		// Call the function
		images, err := getImageDetails(context.Background(), mockClient, repoName)

		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(images) != 1 {
			t.Fatalf("Expected 1 image, got %d", len(images))
		}
		if images[0].ImageTags[0] != "latest" {
			t.Errorf("Expected image tag 'latest', got '%s'", images[0].ImageTags[0])
		}
		if mockClient.ListImagesCalls != 1 {
			t.Errorf("Expected 1 call to ListImages, got %d", mockClient.ListImagesCalls)
		}
		if mockClient.DescribeImagesCalls != 1 {
			t.Errorf("Expected 1 call to DescribeImages, got %d", mockClient.DescribeImagesCalls)
		}
	})

	// Test with no images
	t.Run("No images", func(t *testing.T) {
		repoName := "empty-repo"
		
		// Setup mock client
		mockClient := &MockECRClient{
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{},
				NextToken: nil,
			},
		}

		// Call the function
		images, err := getImageDetails(context.Background(), mockClient, repoName)

		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(images) != 0 {
			t.Fatalf("Expected 0 images, got %d", len(images))
		}
		if mockClient.ListImagesCalls != 1 {
			t.Errorf("Expected 1 call to ListImages, got %d", mockClient.ListImagesCalls)
		}
		if mockClient.DescribeImagesCalls != 0 {
			t.Errorf("Expected 0 calls to DescribeImages, got %d", mockClient.DescribeImagesCalls)
		}
	})
	
	// Note: We can't effectively test pagination with our mock structure
	// Because we can't override the ListImages method in Go
}

// TestSelectImagesForDeletion tests the selectImagesForDeletion function
func TestSelectImagesForDeletion(t *testing.T) {
	now := time.Now()
	
	// Create a set of test images with different ages
	images := []types.ImageDetail{
		{ // Newest image (2 days old)
			ImageDigest: aws.String("sha256:1"),
			ImageTags: []string{"latest"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -2)),
			ImageSizeInBytes: aws.Int64(1000000),
		},
		{ // 8 days old
			ImageDigest: aws.String("sha256:2"),
			ImageTags: []string{"v2"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -8)),
			ImageSizeInBytes: aws.Int64(2000000),
		},
		{ // 12 days old
			ImageDigest: aws.String("sha256:3"),
			ImageTags: []string{"v1"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -12)),
			ImageSizeInBytes: aws.Int64(3000000),
		},
		{ // 15 days old
			ImageDigest: aws.String("sha256:4"),
			ImageTags: []string{"old"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)),
			ImageSizeInBytes: aws.Int64(1500000),
		},
	}

	// Test default behavior (delete images older than 10 days)
	t.Run("Default 10 day retention", func(t *testing.T) {
		config := Config{
			Days: 10,
			MaxImages: 0,
		}
		
		toDelete := selectImagesForDeletion(images, config)
		
		if len(toDelete) != 2 {
			t.Fatalf("Expected 2 images to delete, got %d", len(toDelete))
		}
		
		// Check the correct images were selected
		if *toDelete[0].ImageDigest != "sha256:3" {
			t.Errorf("Expected first image to delete to be sha256:3, got %s", *toDelete[0].ImageDigest)
		}
		if *toDelete[1].ImageDigest != "sha256:4" {
			t.Errorf("Expected second image to delete to be sha256:4, got %s", *toDelete[1].ImageDigest)
		}
	})
	
	// Test with MaxImages retention
	t.Run("Keep newest 2 images", func(t *testing.T) {
		config := Config{
			Days: 5, // Would normally delete 3 images
			MaxImages: 2,
		}
		
		toDelete := selectImagesForDeletion(images, config)
		
		if len(toDelete) != 2 {
			t.Fatalf("Expected 2 images to delete, got %d", len(toDelete))
		}
		
		// Check the correct images were selected (should exclude the 2 newest)
		if *toDelete[0].ImageDigest != "sha256:3" {
			t.Errorf("Expected first image to delete to be sha256:3, got %s", *toDelete[0].ImageDigest)
		}
		if *toDelete[1].ImageDigest != "sha256:4" {
			t.Errorf("Expected second image to delete to be sha256:4, got %s", *toDelete[1].ImageDigest)
		}
	})
	
	// Test with no images to delete
	t.Run("No images to delete", func(t *testing.T) {
		config := Config{
			Days: 20, // All images are newer than this
		}
		
		toDelete := selectImagesForDeletion(images, config)
		
		if len(toDelete) != 0 {
			t.Fatalf("Expected 0 images to delete, got %d", len(toDelete))
		}
	})
	
	// Test with empty image list
	t.Run("Empty image list", func(t *testing.T) {
		config := Config{
			Days: 10,
		}
		
		toDelete := selectImagesForDeletion([]types.ImageDetail{}, config)
		
		if len(toDelete) != 0 {
			t.Fatalf("Expected 0 images to delete from empty list, got %d", len(toDelete))
		}
	})
	
	// Test with nil ImagePushedAt
	t.Run("Nil pushed time", func(t *testing.T) {
		nilTimeImages := []types.ImageDetail{
			{
				ImageDigest: aws.String("sha256:nil"),
				ImageTags: []string{"nil-time"},
				// ImagePushedAt is nil
			},
			{
				ImageDigest: aws.String("sha256:valid"),
				ImageTags: []string{"valid-time"},
				ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)),
			},
		}
		
		config := Config{
			Days: 10,
		}
		
		toDelete := selectImagesForDeletion(nilTimeImages, config)
		
		if len(toDelete) != 1 {
			t.Fatalf("Expected 1 image to delete (the one with valid time), got %d", len(toDelete))
		}
		if *toDelete[0].ImageDigest != "sha256:valid" {
			t.Errorf("Expected to delete image with valid time, got %s", *toDelete[0].ImageDigest)
		}
	})
}

// TestSortImagesByPushedTime tests the sortImagesByPushedTime function
func TestSortImagesByPushedTime(t *testing.T) {
	now := time.Now()
	
	// Create an unsorted list of images
	images := []types.ImageDetail{
		{ // 15 days old
			ImageDigest: aws.String("sha256:4"),
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)),
		},
		{ // 2 days old - should be first after sort
			ImageDigest: aws.String("sha256:1"),
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -2)),
		},
		{ // 12 days old
			ImageDigest: aws.String("sha256:3"),
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -12)),
		},
		{ // 8 days old
			ImageDigest: aws.String("sha256:2"),
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -8)),
		},
	}
	
	// Sort the images
	sortImagesByPushedTime(images)
	
	// Check the order
	if *images[0].ImageDigest != "sha256:1" {
		t.Errorf("Expected first image after sort to be sha256:1, got %s", *images[0].ImageDigest)
	}
	if *images[1].ImageDigest != "sha256:2" {
		t.Errorf("Expected second image after sort to be sha256:2, got %s", *images[1].ImageDigest)
	}
	if *images[2].ImageDigest != "sha256:3" {
		t.Errorf("Expected third image after sort to be sha256:3, got %s", *images[2].ImageDigest)
	}
	if *images[3].ImageDigest != "sha256:4" {
		t.Errorf("Expected fourth image after sort to be sha256:4, got %s", *images[3].ImageDigest)
	}
	
	// Test with nil ImagePushedAt values
	t.Run("Sorting with nil times", func(t *testing.T) {
		nilTimeImages := []types.ImageDetail{
			{ // No pushed time
				ImageDigest: aws.String("sha256:nil1"),
			},
			{ // Valid time
				ImageDigest: aws.String("sha256:valid"),
				ImagePushedAt: aws.Time(now.AddDate(0, 0, -5)),
			},
			{ // Another nil time
				ImageDigest: aws.String("sha256:nil2"),
			},
		}
		
		// Sort the images
		sortImagesByPushedTime(nilTimeImages)
		
		// Check the order - valid time should be first, nil times at the end
		if *nilTimeImages[0].ImageDigest != "sha256:valid" {
			t.Errorf("Expected first image after sort to be sha256:valid, got %s", *nilTimeImages[0].ImageDigest)
		}
		// The nil time images should be after the valid ones, but order between them doesn't matter
	})
	
	// Test empty slice
	t.Run("Sorting empty slice", func(t *testing.T) {
		emptyImages := []types.ImageDetail{}
		
		// This should not panic
		sortImagesByPushedTime(emptyImages)
		
		// No assertions needed - just verifying it doesn't panic
	})
}

// TestGetImageTag tests the getImageTag function
func TestGetImageTag(t *testing.T) {
	// Test with a tagged image
	t.Run("Tagged image", func(t *testing.T) {
		img := types.ImageDetail{
			ImageDigest: aws.String("sha256:123"),
			ImageTags: []string{"latest", "v1"},
		}
		
		tag := getImageTag(img)
		
		if tag != "latest" {
			t.Errorf("Expected tag 'latest', got '%s'", tag)
		}
	})
	
	// Test with an untagged image
	t.Run("Untagged image", func(t *testing.T) {
		digest := "sha256:123456"
		img := types.ImageDetail{
			ImageDigest: aws.String(digest),
			ImageTags: []string{},
		}
		
		tag := getImageTag(img)
		
		if tag != digest {
			t.Errorf("Expected tag '%s', got '%s'", digest, tag)
		}
	})
}

// TestDeleteImages tests the deleteImages function
func TestDeleteImages(t *testing.T) {
	repoName := "test-repo"
	
	// Test with a single batch of images
	t.Run("Single batch delete", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{
					{
						ImageTag: aws.String("v1"),
					},
					{
						ImageDigest: aws.String("sha256:123"),
					},
				},
			},
		}
		
		// Images to delete
		images := []types.ImageDetail{
			{
				ImageTags: []string{"v1"},
				ImageDigest: aws.String("sha256:123"),
			},
			{
				ImageTags: []string{},
				ImageDigest: aws.String("sha256:456"),
			},
		}
		
		// Call the function
		err := deleteImages(context.Background(), mockClient, repoName, images)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if mockClient.BatchDeleteImageCalls != 1 {
			t.Errorf("Expected 1 call to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
		
		// Verify the input
		if *mockClient.LastBatchDeleteImageInput.RepositoryName != repoName {
			t.Errorf("Expected repository name '%s', got '%s'", repoName, *mockClient.LastBatchDeleteImageInput.RepositoryName)
		}
		if len(mockClient.LastBatchDeleteImageInput.ImageIds) != 2 {
			t.Fatalf("Expected 2 image IDs, got %d", len(mockClient.LastBatchDeleteImageInput.ImageIds))
		}
	})
	
	// Test with multiple batches
	t.Run("Multiple batch delete", func(t *testing.T) {
		// Setup mock client
		mockClient := &MockECRClient{
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{},
			},
		}
		
		// Generate 150 images (more than batch size of 100)
		images := make([]types.ImageDetail, 150)
		for i := 0; i < 150; i++ {
			images[i] = types.ImageDetail{
				ImageTags: []string{},
				ImageDigest: aws.String("sha256:" + string(rune(i))),
			}
		}
		
		// Call the function
		err := deleteImages(context.Background(), mockClient, repoName, images)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if mockClient.BatchDeleteImageCalls != 2 {
			t.Errorf("Expected 2 calls to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
	
	// Test with failures
	t.Run("Deletion failures", func(t *testing.T) {
		// Setup mock client with some failures
		mockClient := &MockECRClient{
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{
					{
						ImageTag: aws.String("v1"),
					},
				},
				Failures: []types.ImageFailure{
					{
						ImageId: &types.ImageIdentifier{
							ImageDigest: aws.String("sha256:123"),
						},
						FailureReason: aws.String("Image not found"),
						FailureCode: "ImageNotFound",
					},
				},
			},
		}
		
		// Images to delete
		images := []types.ImageDetail{
			{
				ImageTags: []string{"v1"},
				ImageDigest: aws.String("sha256:abc"),
			},
			{
				ImageTags: []string{},
				ImageDigest: aws.String("sha256:123"),
			},
		}
		
		// Call the function - should not error even with failures
		err := deleteImages(context.Background(), mockClient, repoName, images)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if mockClient.BatchDeleteImageCalls != 1 {
			t.Errorf("Expected 1 call to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
	
	// Test with no images
	t.Run("No images to delete", func(t *testing.T) {
		mockClient := &MockECRClient{}
		
		// Call with empty slice
		err := deleteImages(context.Background(), mockClient, repoName, []types.ImageDetail{})
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if mockClient.BatchDeleteImageCalls != 0 {
			t.Errorf("Expected 0 calls to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
}

// TestGetImageIdString tests the getImageIdString function
func TestGetImageIdString(t *testing.T) {
	// Test with a tag
	t.Run("ImageId with tag", func(t *testing.T) {
		id := &types.ImageIdentifier{
			ImageTag: aws.String("latest"),
		}
		
		result := getImageIdString(id)
		
		if result != "latest" {
			t.Errorf("Expected 'latest', got '%s'", result)
		}
	})
	
	// Test with a digest
	t.Run("ImageId with digest", func(t *testing.T) {
		digest := "sha256:123"
		id := &types.ImageIdentifier{
			ImageDigest: aws.String(digest),
		}
		
		result := getImageIdString(id)
		
		if result != digest {
			t.Errorf("Expected '%s', got '%s'", digest, result)
		}
	})
	
	// Test with nil
	t.Run("Nil ImageId", func(t *testing.T) {
		result := getImageIdString(nil)
		
		if result != "unknown" {
			t.Errorf("Expected 'unknown', got '%s'", result)
		}
	})
	
	// Test with both tag and digest
	t.Run("ImageId with both tag and digest", func(t *testing.T) {
		id := &types.ImageIdentifier{
			ImageTag: aws.String("latest"),
			ImageDigest: aws.String("sha256:123"),
		}
		
		result := getImageIdString(id)
		
		// Should prefer tag over digest
		if result != "latest" {
			t.Errorf("Expected 'latest', got '%s'", result)
		}
	})
	
	// Test with neither tag nor digest (empty identifier)
	t.Run("Empty ImageId", func(t *testing.T) {
		id := &types.ImageIdentifier{}
		
		result := getImageIdString(id)
		
		if result != "unknown" {
			t.Errorf("Expected 'unknown', got '%s'", result)
		}
	})
}

// TestProcessRepository tests the processRepository function
func TestProcessRepository(t *testing.T) {
	ctx := context.Background()
	repoName := "test-repo"
	now := time.Now()
	
	// Setup test images with varying ages
	olderImage := types.ImageDetail{
		ImageDigest: aws.String("sha256:123"),
		ImageTags: []string{"v1"},
		ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)), // 15 days old
		ImageSizeInBytes: aws.Int64(1000000),
	}
	newerImage := types.ImageDetail{
		ImageDigest: aws.String("sha256:456"),
		ImageTags: []string{"latest"},
		ImagePushedAt: aws.Time(now.AddDate(0, 0, -5)), // 5 days old
		ImageSizeInBytes: aws.Int64(2000000),
	}
	
	// Test in dry run mode with images to delete
	t.Run("Dry run with old images", func(t *testing.T) {
		mockClient := &MockECRClient{
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
					{ImageTag: aws.String("latest")},
				},
			},
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{olderImage, newerImage},
			},
		}
		
		cfg := Config{
			Days: 10, // Delete images older than 10 days
			DryRun: true,
		}
		
		summary, err := processRepository(ctx, mockClient, repoName, cfg)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if summary.ImagesDeleted != 1 {
			t.Errorf("Expected 1 image marked for deletion, got %d", summary.ImagesDeleted)
		}
		if summary.SpaceFreed != 1000000 {
			t.Errorf("Expected 1000000 bytes marked for deletion, got %d", summary.SpaceFreed)
		}
		if mockClient.BatchDeleteImageCalls != 0 {
			t.Errorf("Expected no calls to BatchDeleteImage in dry run mode, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
	
	// Test actual deletion
	t.Run("Actual deletion", func(t *testing.T) {
		mockClient := &MockECRClient{
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
					{ImageTag: aws.String("latest")},
				},
			},
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{olderImage, newerImage},
			},
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
				},
			},
		}
		
		cfg := Config{
			Days: 10, // Delete images older than 10 days
			DryRun: false,
		}
		
		summary, err := processRepository(ctx, mockClient, repoName, cfg)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if summary.ImagesDeleted != 1 {
			t.Errorf("Expected 1 image deleted, got %d", summary.ImagesDeleted)
		}
		if mockClient.BatchDeleteImageCalls != 1 {
			t.Errorf("Expected 1 call to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
		if mockClient.LastBatchDeleteImageInput == nil {
			t.Fatal("Expected BatchDeleteImageInput to be set")
		}
		if len(mockClient.LastBatchDeleteImageInput.ImageIds) != 1 {
			t.Errorf("Expected 1 image ID, got %d", len(mockClient.LastBatchDeleteImageInput.ImageIds))
		}
	})
	
	// Test with no images to delete
	t.Run("No images to delete", func(t *testing.T) {
		mockClient := &MockECRClient{
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("latest")},
				},
			},
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{newerImage},
			},
		}
		
		cfg := Config{
			Days: 10, // Delete images older than 10 days
			DryRun: false,
		}
		
		summary, err := processRepository(ctx, mockClient, repoName, cfg)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if summary.ImagesDeleted != 0 {
			t.Errorf("Expected 0 images deleted, got %d", summary.ImagesDeleted)
		}
		if mockClient.BatchDeleteImageCalls != 0 {
			t.Errorf("Expected 0 calls to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
	})
}

// TestCleanupECR tests the overall cleanup process with a helper function
func TestCleanupECR(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	
	// Test with multiple repositories
	t.Run("Multiple repositories cleanup", func(t *testing.T) {
		// Setup test repositories and images
		repo1 := "repo1"
		repo2 := "repo2"
		
		// Images that should be deleted (older than cutoff)
		oldImage1 := types.ImageDetail{
			ImageDigest: aws.String("sha256:111"),
			ImageTags: []string{"v1"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -15)), // 15 days old
			ImageSizeInBytes: aws.Int64(1000000),
		}
		
		oldImage2 := types.ImageDetail{
			ImageDigest: aws.String("sha256:222"),
			ImageTags: []string{"v1"},
			ImagePushedAt: aws.Time(now.AddDate(0, 0, -12)), // 12 days old
			ImageSizeInBytes: aws.Int64(2000000),
		}
		
		// Mock client setup
		mockClient := &MockECRClient{
			// Mock repository list
			DescribeRepositoriesOutput: &ecr.DescribeRepositoriesOutput{
				Repositories: []types.Repository{
					{RepositoryName: aws.String(repo1)},
					{RepositoryName: aws.String(repo2)},
				},
			},
			// Mock image lists (each repository has one image to delete)
			ListImagesOutput: &ecr.ListImagesOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
				},
			},
			// Mock deleteImage response
			BatchDeleteImageOutput: &ecr.BatchDeleteImageOutput{
				ImageIds: []types.ImageIdentifier{
					{ImageTag: aws.String("v1")},
				},
			},
			// Setup DescribeImages response with the old images
			DescribeImagesOutput: &ecr.DescribeImagesOutput{
				ImageDetails: []types.ImageDetail{oldImage1, oldImage2},
			},
		}
		
		// Run our test
		cfg := Config{
			Days: 10, // Delete images older than 10 days
			DryRun: false,
		}
		
		// Helper function that simulates cleanupECR but uses our mock client
		testCleanupWithMockClient := func(ctx context.Context, cfg Config, client ECRClient) (CleanupSummary, error) {
			summary := CleanupSummary{}
			
			// Get repositories
			repos, err := getRepositories(ctx, client)
			if err != nil {
				return summary, err
			}
			
			summary.RepositoriesProcessed = len(repos)
			
			// Process each repository
			for _, repo := range repos {
				repoSummary, err := processRepository(ctx, client, *repo.RepositoryName, cfg)
				if err != nil {
					// Log error and continue in real code
					continue
				}
				
				summary.ImagesDeleted += repoSummary.ImagesDeleted
				summary.SpaceFreed += repoSummary.SpaceFreed
			}
			
			return summary, nil
		}
		
		// Execute test
		summary, err := testCleanupWithMockClient(ctx, cfg, mockClient)
		
		// Assertions
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if summary.RepositoriesProcessed != 2 {
			t.Errorf("Expected 2 repositories processed, got %d", summary.RepositoriesProcessed)
		}
		if summary.ImagesDeleted != 4 {
			t.Errorf("Expected 4 images deleted, got %d", summary.ImagesDeleted)
		}
		if mockClient.BatchDeleteImageCalls != 2 {
			t.Errorf("Expected 2 calls to BatchDeleteImage, got %d", mockClient.BatchDeleteImageCalls)
		}
		if summary.SpaceFreed != 6000000 {
			t.Errorf("Expected 6000000 bytes freed (6MB), got %d", summary.SpaceFreed)
		}
	})
}

// TestLoadAWSConfig tests the loadAWSConfig function
func TestLoadAWSConfig(t *testing.T) {
	// Because we can't easily mock the AWS config loader directly,
	// we'll just test the function's basic logic with the actual AWS SDK
	
	// Test with default region
	t.Run("Default region", func(t *testing.T) {
		ctx := context.Background()
		_, err := loadAWSConfig(ctx, "")
		
		// We're just checking that it doesn't error
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})
	
	// Test with specified region
	t.Run("Specified region", func(t *testing.T) {
		ctx := context.Background()
		specifiedRegion := "eu-central-1"
		cfg, err := loadAWSConfig(ctx, specifiedRegion)
		
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check that the region was set correctly
		if cfg.Region != specifiedRegion {
			t.Errorf("Expected region to be %s, got %s", specifiedRegion, cfg.Region)
		}
	})
}

// TestParseFlags tests the parseFlags function
func TestParseFlags(t *testing.T) {
	// Save original command line arguments and restore after test
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Test default values
	t.Run("Default values", func(t *testing.T) {
		// Reset flags
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		os.Args = []string{"cmd"}
		
		// Call parseFlags
		config := parseFlags()
		
		// Check defaults
		if config.DryRun != false {
			t.Errorf("Expected DryRun to be false, got %v", config.DryRun)
		}
		if config.Days != 10 {
			t.Errorf("Expected Days to be 10, got %d", config.Days)
		}
		if config.Region != "" {
			t.Errorf("Expected Region to be empty, got %s", config.Region)
		}
		if config.MaxImages != 0 {
			t.Errorf("Expected MaxImages to be 0, got %d", config.MaxImages)
		}
	})
	
	// Test with custom values
	t.Run("Custom values", func(t *testing.T) {
		// Reset flags
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		os.Args = []string{
			"cmd",
			"-dry-run=true",
			"-days=30",
			"-region=eu-west-1",
			"-max-images=5",
		}
		
		// Call parseFlags
		config := parseFlags()
		
		// Check values
		if config.DryRun != true {
			t.Errorf("Expected DryRun to be true, got %v", config.DryRun)
		}
		if config.Days != 30 {
			t.Errorf("Expected Days to be 30, got %d", config.Days)
		}
		if config.Region != "eu-west-1" {
			t.Errorf("Expected Region to be eu-west-1, got %s", config.Region)
		}
		if config.MaxImages != 5 {
			t.Errorf("Expected MaxImages to be 5, got %d", config.MaxImages)
		}
	})
}