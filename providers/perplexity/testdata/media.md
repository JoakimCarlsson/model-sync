> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.
# Media & Attachments
> Send and receive images, videos, and files with the Sonar API
## Overview
The Sonar API supports comprehensive media handling: send images and files for analysis, and receive images and videos in responses. This guide covers all media functionality in one place.
## Sending Images
  * Base64 images: Maximum 50 MB per image. Supported formats: PNG, JPEG, WEBP, GIF
  * HTTPS URLs: Must be publicly accessible and point directly to the image file
### Base64 Encoded Images
# Read and encode image as base64
# Analyze the image
### HTTPS URL Images
### Key Parameters
## Sending Files
### Using a Public URL
### Using Base64 Encoding
# Read and encode file (no prefix needed)
### Key Parameters
## Receiving Images
### Basic Image Returns
### Filtering Image Domains
# Exclude specific domains (prefix with -)
# Include only specific domains
### Filtering Image Formats
# Only return GIF images
# Allow multiple formats
### Key Parameters
## Receiving Videos
### Basic Video Returns
### Combining Videos with Images
### Key Parameters
## Best Practices
## Next Steps
