# ECR Image Cleanup Tool

A Go-based utility to automatically clean up old images in Amazon ECR repositories.

## Overview

This tool scans your AWS ECR repositories and deletes images that are older than a specified number of days. It helps you manage storage costs and keep your repositories organized by automatically removing outdated container images.

Key features:
- Clean up images older than X days (default: 10 days)
- Option to keep a minimum number of images per repository regardless of age
- Dry-run mode to preview what would be deleted
- Region selection support
- Detailed reporting showing space recovered

## Requirements

- Go 1.22 or later
- AWS credentials with ECR permissions:
  - `ecr:DescribeRepositories`
  - `ecr:ListImages`
  - `ecr:DescribeImages`
  - `ecr:BatchDeleteImage`

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/ecr-cleanup.git
cd ecr-cleanup

# Build the executable
go build -o ecr-cleanup

# Move to a location in your PATH (optional)
sudo mv ecr-cleanup /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/mchineboy/ecr-cleanup@latest
```

## Usage

```
./ecr-cleanup [options]
```

### Command-line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-days` | Delete images older than this many days | 10 |
| `-dry-run` | Preview which images would be deleted without actually removing them | false |
| `-max-images` | Keep at least this many newest images per repository | 0 (no limit) |
| `-region` | AWS region to use | (from AWS config) |

### Examples

#### Basic cleanup (10-day retention)

```bash
./ecr-cleanup
```

#### Preview what would be deleted without actually deleting

```bash
./ecr-cleanup -dry-run
```

#### Delete images older than 30 days

```bash
./ecr-cleanup -days 30
```

#### Keep at least 5 images per repository

```bash
./ecr-cleanup -max-images 5
```

#### Specify a different AWS region

```bash
./ecr-cleanup -region us-west-2
```

#### Combined options

```bash
./ecr-cleanup -days 14 -max-images 3 -region eu-central-1
```

## AWS Credentials

The tool uses the standard AWS credentials chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role for Amazon EC2 or ECS task role

Make sure your credentials are properly configured before running the tool. You can use the AWS CLI to configure your credentials:

```bash
aws configure
```

## Example Output

```
2025/05/13 14:32:33 Found 5 repositories
2025/05/13 14:32:33 Processing repository: myapp-prod
2025/05/13 14:32:33 Found 12 images in repository myapp-prod
2025/05/13 14:32:33 Selected 9 images for deletion in repository myapp-prod
2025/05/13 14:32:33 Deleted 9 images from repository myapp-prod
2025/05/13 14:32:33 Processing repository: myapp-staging
2025/05/13 14:32:33 Found 24 images in repository myapp-staging
2025/05/13 14:32:33 Selected 20 images for deletion in repository myapp-staging
2025/05/13 14:32:33 Deleted 20 images from repository myapp-staging
2025/05/13 14:32:33 ECR Cleanup Summary:
2025/05/13 14:32:33 - Repositories processed: 5
2025/05/13 14:32:33 - Images deleted: 32
2025/05/13 14:32:33 - Space freed: 2546.25 MB
```

## Scheduling with Cron

To run the cleanup tool automatically on a schedule, you can use cron:

```bash
# Edit your crontab
crontab -e

# Add a line to run the script weekly (Sunday at 1 AM)
0 1 * * 0 /path/to/ecr-cleanup -days 30 >> /var/log/ecr-cleanup.log 2>&1
```

## Development

### Running Tests

The project includes a comprehensive test suite:

```bash
# Run all tests
go test

# Run tests with verbose output
go test -v

# Check test coverage
go test -cover

# Generate detailed coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Project Structure

```
├── main.go         # Main application code
├── main_test.go    # Test suite
├── go.mod          # Go module definition
├── go.sum          # Module checksums
└── README.md       # Documentation
```

## License

[MIT License](LICENSE)