package cloudinary

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// Client wraps the Cloudinary client
type Client struct {
	cld *cloudinary.Cloudinary
	ctx context.Context
}

// NewClient creates a new Cloudinary client
// The CLOUDINARY_URL environment variable should be set in the format:
// cloudinary://API_KEY:API_SECRET@CLOUD_NAME
func NewClient() (*Client, error) {
	cld, err := cloudinary.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	// Ensure secure URLs (https)
	cld.Config.URL.Secure = true

	return &Client{
		cld: cld,
		ctx: context.Background(),
	}, nil
}

// UploadImage uploads an image file to Cloudinary
// Returns the secure URL of the uploaded image
func (c *Client) UploadImage(file multipart.File, filename string, folder string) (string, error) {
	// Validate file type
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExtensions[ext] {
		return "", fmt.Errorf("invalid file type: %s. Allowed types: jpg, jpeg, png, gif, webp", ext)
	}

	// Read the file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Generate a public ID from the filename (without extension)
	publicID := strings.TrimSuffix(filename, ext)
	if folder != "" {
		publicID = folder + "/" + publicID
	}

	// Helper variables for boolean pointers
	uniqueFilename := true
	overwrite := false

	// Use a reader for the SDK (passing []byte directly can be treated as unsupported by the SDK)
	reader := bytes.NewReader(fileBytes)

	// Upload to Cloudinary
	uploadResult, err := c.cld.Upload.Upload(c.ctx, reader, uploader.UploadParams{
		PublicID:       publicID,
		Folder:         folder,
		ResourceType:   "image",
		UniqueFilename: &uniqueFilename,
		Overwrite:      &overwrite,
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, nil
}

// DeleteImage deletes an image from Cloudinary by public ID
func (c *Client) DeleteImage(publicID string) error {
	_, err := c.cld.Upload.Destroy(c.ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: "image",
	})

	if err != nil {
		return fmt.Errorf("failed to delete from Cloudinary: %w", err)
	}

	return nil
}

// ExtractPublicID extracts the public ID from a Cloudinary URL
// Example: https://res.cloudinary.com/demo/image/upload/v1234567890/products/sample.jpg
// Returns: products/sample
func ExtractPublicID(url string) string {
	if url == "" {
		return ""
	}

	// Split by "upload/" to get the part after it
	parts := strings.Split(url, "/upload/")
	if len(parts) < 2 {
		return ""
	}

	// Get the path after upload/
	path := parts[1]

	// Remove version prefix (v1234567890/)
	pathParts := strings.Split(path, "/")
	if len(pathParts) > 1 && strings.HasPrefix(pathParts[0], "v") {
		path = strings.Join(pathParts[1:], "/")
	}

	// Remove file extension
	ext := filepath.Ext(path)
	publicID := strings.TrimSuffix(path, ext)

	return publicID
}
