package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const segmentationDownloadTimeout = 30 * time.Second
const segmentationMaxDownloadBytes = 128 * 1024 * 1024

type segmentationModelConfig struct {
	InputImageField string         `json:"input_image_field"`
	InputDefaults   map[string]any `json:"input_defaults"`
	ResultPath      string         `json:"result_path"`
	ResultSource    string         `json:"result_source"`
	OutputImagePath string         `json:"output_image_path"`
}

func parseSegmentationModelConfig(raw string) (segmentationModelConfig, error) {
	cfg := segmentationModelConfig{}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "{}"
	}
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return segmentationModelConfig{}, fmt.Errorf("invalid segmentation model config: %w", err)
	}
	if strings.TrimSpace(cfg.InputImageField) == "" {
		cfg.InputImageField = "image"
	}
	if cfg.InputDefaults == nil {
		cfg.InputDefaults = map[string]any{}
	}
	if strings.TrimSpace(cfg.ResultPath) == "" {
		cfg.ResultPath = "$"
	}
	if strings.TrimSpace(cfg.ResultSource) == "" {
		cfg.ResultSource = "raw"
	}
	return cfg, nil
}

func extractValueByPath(payload any, path string) (any, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" || trimmedPath == "$" {
		return payload, nil
	}

	current := payload
	for _, part := range strings.Split(trimmedPath, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q expected object, got %T", trimmedPath, current)
		}

		next, exists := object[part]
		if !exists {
			return nil, fmt.Errorf("path %q missing key %q", trimmedPath, part)
		}
		current = next
	}

	return current, nil
}

func extractStringByPath(payload any, path string) (string, error) {
	value, err := extractValueByPath(payload, path)
	if err != nil {
		return "", err
	}

	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("path %q is not a string: %T", path, value)
	}
	return strings.TrimSpace(text), nil
}

func resolveSegmentationResult(ctx context.Context, rawOutput any, cfg segmentationModelConfig) (any, error) {
	value, err := extractValueByPath(rawOutput, cfg.ResultPath)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ResultSource)) {
	case "", "raw":
		return value, nil
	case "url_json":
		resultURL, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("result path %q is not a string url: %T", cfg.ResultPath, value)
		}
		body, _, err := downloadURL(ctx, resultURL, segmentationMaxDownloadBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to download segmentation json: %w", err)
		}
		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse segmentation json: %w", err)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported segmentation result_source %q", cfg.ResultSource)
	}
}

func marshalJSON(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func downloadURL(ctx context.Context, rawURL string, maxBytes int) ([]byte, string, error) {
	if maxBytes <= 0 {
		maxBytes = segmentationMaxDownloadBytes
	}
	requestCtx, cancel := context.WithTimeout(ctx, segmentationDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("failed to download %q: status=%d", rawURL, resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, int64(maxBytes)+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read download body: %w", err)
	}
	if len(body) > maxBytes {
		return nil, "", fmt.Errorf("download exceeded max bytes (%d)", maxBytes)
	}

	return body, resp.Header.Get("Content-Type"), nil
}
