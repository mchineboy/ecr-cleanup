package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRClient defines an interface for ECR operations
// This makes testing easier by allowing us to mock the AWS service
type ECRClient interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
	ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error)
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
	BatchDeleteImage(ctx context.Context, params *ecr.BatchDeleteImageInput, optFns ...func(*ecr.Options)) (*ecr.BatchDeleteImageOutput, error)
}

// Config holds the application configuration
type Config struct {
	DryRun    bool
	Days      int
	Region    string
	MaxImages int
}

// CleanupSummary tracks the results of the cleanup operation
type CleanupSummary struct {
	RepositoriesProcessed int
	ImagesDeleted         int
	SpaceFreed            int64 // in bytes
}

// main is the entry point for the application
func main() {
	// Parse command line arguments
	config := parseFlags()

	summary, err := cleanupECR(config)
	if err != nil {
		log.Fatalf("Error cleaning up ECR repositories: %v", err)
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
}

// parseFlags parses command line flags and returns the configuration
func parseFlags() Config {
	dryRun := flag.Bool("dry-run", false, "Dry run mode (don't actually delete images)")
	days := flag.Int("days", 10, "Delete images older than this many days")
	region := flag.String("region", "", "AWS region (defaults to value from AWS config)")
	maxImages := flag.Int("max-images", 0, "Maximum number of images to keep per repository (0 means no limit)")

	flag.Parse()

	return Config{
		DryRun:    *dryRun,
		Days:      *days,
		Region:    *region,
		MaxImages: *maxImages,
	}
}

// cleanupECR performs the ECR cleanup operation
func cleanupECR(cfg Config) (CleanupSummary, error) {
	summary := CleanupSummary{}
	ctx := context.Background()

	// Load AWS configuration
	awsConfig, err := loadAWSConfig(ctx, cfg.Region)
	if err != nil {
		return summary, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECR client
	client := ecr.NewFromConfig(awsConfig)

	// Get all repositories
	repos, err := getRepositories(ctx, client)
	if err != nil {
		return summary, fmt.Errorf("failed to get repositories: %w", err)
	}
	
	summary.RepositoriesProcessed = len(repos)

	log.Printf("Found %d repositories", len(repos))

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

// loadAWSConfig loads the AWS configuration
func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	configOpts := []func(*config.LoadOptions) error{}
	if region != "" {
		configOpts = append(configOpts, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(ctx, configOpts...)
}

// getRepositories gets all ECR repositories
func getRepositories(ctx context.Context, client ECRClient) ([]types.Repository, error) {
	var repositories []types.Repository
	var nextToken *string

	for {
		resp, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		repositories = append(repositories, resp.Repositories...)

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}

	return repositories, nil
}

// processRepository processes a single ECR repository
func processRepository(ctx context.Context, client ECRClient, repoName string, cfg Config) (CleanupSummary, error) {
	repoSummary := CleanupSummary{RepositoriesProcessed: 1}
	log.Printf("Processing repository: %s", repoName)

	// Get all image details
	images, err := getImageDetails(ctx, client, repoName)
	if err != nil {
		return repoSummary, fmt.Errorf("failed to get image details: %w", err)
	}

	log.Printf("Found %d images in repository %s", len(images), repoName)

	// Determine which images to delete
	toDelete := selectImagesForDeletion(images, cfg)

	if len(toDelete) == 0 {
		log.Printf("No images to delete in repository %s", repoName)
		return repoSummary, nil
	}
	
	repoSummary.ImagesDeleted = len(toDelete)
	
	// Calculate space to be freed
	for _, img := range toDelete {
		if img.ImageSizeInBytes != nil {
			repoSummary.SpaceFreed += *img.ImageSizeInBytes
		}
	}

	log.Printf("Selected %d images for deletion in repository %s", len(toDelete), repoName)

	// If in dry run mode, just print what would be deleted
	if cfg.DryRun {
		for _, img := range toDelete {
			pushedAtStr := "unknown time"
			if img.ImagePushedAt != nil {
				pushedAtStr = img.ImagePushedAt.Format(time.RFC3339)
			}
			
			sizeStr := "unknown size"
			if img.ImageSizeInBytes != nil {
				sizeStr = fmt.Sprintf("%.2f MB", float64(*img.ImageSizeInBytes)/1024/1024)
			}
			
			log.Printf("[DRY RUN] Would delete image %s:%s (pushed at %s, size: %s)",
				repoName, getImageTag(img), pushedAtStr, sizeStr)
		}
		return repoSummary, nil
	}

	// Delete the images
	err = deleteImages(ctx, client, repoName, toDelete)
	if err != nil {
		return repoSummary, err
	}
	
	return repoSummary, nil
}

// getImageDetails gets details for all images in a repository
func getImageDetails(ctx context.Context, client ECRClient, repoName string) ([]types.ImageDetail, error) {
	var images []types.ImageDetail
	var nextToken *string

	for {
		// First, get the image IDs
		listResp, err := client.ListImages(ctx, &ecr.ListImagesInput{
			RepositoryName: aws.String(repoName),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, err
		}

		// Get detailed information about these images
		if len(listResp.ImageIds) > 0 {
			descResp, err := client.DescribeImages(ctx, &ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				ImageIds:       listResp.ImageIds,
			})
			if err != nil {
				return nil, err
			}

			images = append(images, descResp.ImageDetails...)
		}

		nextToken = listResp.NextToken
		if nextToken == nil {
			break
		}
	}

	return images, nil
}

// selectImagesForDeletion determines which images should be deleted
func selectImagesForDeletion(images []types.ImageDetail, cfg Config) []types.ImageDetail {
	cutoffTime := time.Now().AddDate(0, 0, -cfg.Days)
	var toDelete []types.ImageDetail

	// Sort images by pushed time (newest first)
	sortImagesByPushedTime(images)

	// If maxImages is set, keep the newest N images
	keepCount := 0
	if cfg.MaxImages > 0 {
		keepCount = cfg.MaxImages
		if keepCount > len(images) {
			keepCount = len(images)
		}
	}

	for i, img := range images {
		// Skip the newest N images if maxImages is set
		if i < keepCount {
			continue
		}

		// Delete images older than the cutoff time
		if img.ImagePushedAt != nil && img.ImagePushedAt.Before(cutoffTime) {
			toDelete = append(toDelete, img)
		}
	}

	return toDelete
}

// sortImagesByPushedTime sorts images by pushed time (newest first)
func sortImagesByPushedTime(images []types.ImageDetail) {
	// Sort by pushed time (newest first) using sort.Slice for better performance
	sort.Slice(images, func(i, j int) bool {
		// Handle nil pointers gracefully
		if images[i].ImagePushedAt == nil {
			return false // nil times sort to the end
		}
		if images[j].ImagePushedAt == nil {
			return true // nil times sort to the end
		}
		// Sort newest first (reverse chronological order)
		return images[i].ImagePushedAt.After(*images[j].ImagePushedAt)
	})
}

// getImageTag returns a tag for the image (or digest if no tags)
func getImageTag(img types.ImageDetail) string {
	if len(img.ImageTags) > 0 {
		return img.ImageTags[0]
	}
	// If no tags, use digest
	return *img.ImageDigest
}

// deleteImages deletes the specified images from the repository
func deleteImages(ctx context.Context, client ECRClient, repoName string, images []types.ImageDetail) error {
	// AWS API has a limit of 100 images per batch delete operation
	const batchSize = 100

	for i := 0; i < len(images); i += batchSize {
		end := i + batchSize
		if end > len(images) {
			end = len(images)
		}

		batch := images[i:end]
		imageIds := make([]types.ImageIdentifier, len(batch))

		for j, img := range batch {
			// Prefer tag if available, otherwise use digest
			if len(img.ImageTags) > 0 {
				imageIds[j] = types.ImageIdentifier{
					ImageTag: aws.String(img.ImageTags[0]),
				}
			} else {
				imageIds[j] = types.ImageIdentifier{
					ImageDigest: img.ImageDigest,
				}
			}
		}

		result, err := client.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
			RepositoryName: aws.String(repoName),
			ImageIds:       imageIds,
		})
		if err != nil {
			return fmt.Errorf("failed to delete batch of images: %w", err)
		}

		log.Printf("Deleted %d images from repository %s", len(batch), repoName)
		
		// Log any failures
		if len(result.Failures) > 0 {
			for _, failure := range result.Failures {
				log.Printf("Failed to delete image: %s, reason: %s, code: %s",
					getImageIdString(failure.ImageId),
					*failure.FailureReason,
					string(failure.FailureCode))
			}
		}
	}

	return nil
}

// getImageIdString creates a string representation of an ImageIdentifier
func getImageIdString(id *types.ImageIdentifier) string {
	if id == nil {
		return "unknown"
	}
	if id.ImageTag != nil {
		return *id.ImageTag
	}
	if id.ImageDigest != nil {
		return *id.ImageDigest
	}
	return "unknown"
}
