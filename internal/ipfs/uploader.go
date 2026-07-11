package ipfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Uploader struct {
	url        string
	httpClient *http.Client
}

func NewUploader(url string) *Uploader {
	return &Uploader{
		url: strings.TrimSpace(url),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// UploadMetadata uploads a market metadata JSON document to either the local
// content service or a Kubo-compatible IPFS add endpoint.
func (u *Uploader) UploadMetadata(ctx context.Context, metadataJSON string) (string, error) {
	if u == nil || strings.TrimSpace(u.url) == "" {
		return "", fmt.Errorf("IPFS upload URL is not configured")
	}
	if strings.TrimSpace(metadataJSON) == "" {
		return "", fmt.Errorf("market metadata is empty")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "market.json")
	if err != nil {
		return "", fmt.Errorf("create IPFS metadata form: %w", err)
	}
	if _, err := io.WriteString(file, metadataJSON); err != nil {
		return "", fmt.Errorf("write IPFS metadata form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finalize IPFS metadata form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.url, &body)
	if err != nil {
		return "", fmt.Errorf("create IPFS upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload IPFS metadata: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read IPFS upload response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("IPFS metadata upload returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var result struct {
		Hash string `json:"Hash"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("parse IPFS upload response: %w", err)
	}
	if strings.TrimSpace(result.Hash) == "" {
		return "", fmt.Errorf("IPFS upload response is missing Hash")
	}
	return strings.TrimSpace(result.Hash), nil
}
