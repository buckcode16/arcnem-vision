package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/replicate/replicate-go"
)

const clipRequestTimeout = 90 * time.Second
const clipPredictionURL = "https://api.replicate.com/v1/models/openai/clip/predictions"
const clipPreferWaitSeconds = "60"
const clipResponsePreviewLimit = 800

func runCLIPPrediction(ctx context.Context, input map[string]any) (map[string]any, error) {
	runCtx, cancel := context.WithTimeout(ctx, clipRequestTimeout)
	defer cancel()

	token := strings.TrimSpace(os.Getenv("REPLICATE_API_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("REPLICATE_API_TOKEN not set")
	}

	normalizedInput, err := normalizeCLIPInput(input)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]any{
		"input": normalizedInput,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal replicate request: %w", err)
	}

	payload, err := doReplicateJSONRequest(
		runCtx,
		token,
		http.MethodPost,
		clipPredictionURL,
		body,
		clipPreferWaitSeconds,
	)
	if err != nil {
		return nil, err
	}
	payload, err = waitForReplicatePrediction(runCtx, token, payload)
	if err != nil {
		return nil, err
	}

	outputRaw, ok := payload["output"]
	if !ok || outputRaw == nil {
		return nil, fmt.Errorf("replicate response missing output")
	}

	output, ok := outputRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("replicate output is not an object: %T", outputRaw)
	}

	return output, nil
}

func normalizeCLIPInput(input map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(input))
	for key, value := range input {
		if file, ok := value.(*replicate.File); ok {
			if file == nil {
				return nil, fmt.Errorf("replicate input %q file is nil", key)
			}
			fileURL := strings.TrimSpace(file.URLs["get"])
			if fileURL == "" {
				return nil, fmt.Errorf("replicate input %q file has empty get url", key)
			}
			normalized[key] = fileURL
			continue
		}
		normalized[key] = value
	}
	return normalized, nil
}

func responsePreview(body []byte) string {
	text := string(bytes.ToValidUTF8(body, []byte{}))
	text = strings.TrimSpace(text)
	if text == "" {
		return "<empty>"
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > clipResponsePreviewLimit {
		return text[:clipResponsePreviewLimit] + "..."
	}
	return text
}
