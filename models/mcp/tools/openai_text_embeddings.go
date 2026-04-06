package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const openAIEmbeddingRequestTimeout = 60 * time.Second
const openAIEmbeddingEndpoint = "https://api.openai.com/v1/embeddings"
const openAITextEmbeddingModelName = "text-embedding-3-large"
const openAITextEmbeddingProvider = "OPENAI"
const openAITextEmbeddingDim = 1536
const openAIEmbeddingBatchSize = 64
const openAIEmbeddingMaxChars = 12000
const openAIEmbeddingResponsePreviewLimit = 600

type openAIEmbeddingsRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions"`
	EncodingFormat string   `json:"encoding_format"`
}

type openAIEmbeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func buildOpenAITextEmbedding(ctx context.Context, input string) (string, error) {
	embeddings, err := buildOpenAITextEmbeddings(ctx, []string{input})
	if err != nil {
		return "", err
	}
	if len(embeddings) == 0 {
		return "", nil
	}
	return embeddings[0], nil
}

func buildOpenAITextEmbeddings(ctx context.Context, inputs []string) ([]string, error) {
	if len(inputs) == 0 {
		return []string{}, nil
	}

	token := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if token == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	normalizedInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		normalized := normalizeEmbeddingText(input)
		if normalized == "" {
			return nil, fmt.Errorf("embedding input is empty after normalization")
		}
		normalizedInputs = append(normalizedInputs, normalized)
	}

	embeddings := make([]string, 0, len(normalizedInputs))
	for start := 0; start < len(normalizedInputs); start += openAIEmbeddingBatchSize {
		end := min(start+openAIEmbeddingBatchSize, len(normalizedInputs))
		batchEmbeddings, err := requestOpenAITextEmbeddingBatch(
			ctx,
			token,
			normalizedInputs[start:end],
		)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, batchEmbeddings...)
	}

	return embeddings, nil
}

func normalizeEmbeddingText(input string) string {
	normalized := normalizeWhitespace(string(bytes.ToValidUTF8([]byte(input), []byte{})))
	if normalized == "" {
		return ""
	}

	runes := []rune(normalized)
	if len(runes) <= openAIEmbeddingMaxChars {
		return normalized
	}
	return strings.TrimSpace(string(runes[:openAIEmbeddingMaxChars]))
}

func requestOpenAITextEmbeddingBatch(
	ctx context.Context,
	token string,
	inputs []string,
) ([]string, error) {
	runCtx, cancel := context.WithTimeout(ctx, openAIEmbeddingRequestTimeout)
	defer cancel()

	body, err := json.Marshal(openAIEmbeddingsRequest{
		Model:          openAITextEmbeddingModelName,
		Input:          inputs,
		Dimensions:     openAITextEmbeddingDim,
		EncodingFormat: "float",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI embeddings request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		runCtx,
		http.MethodPost,
		openAIEmbeddingEndpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI embeddings request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI embeddings request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"OpenAI embeddings request failed with status %d: %s",
			resp.StatusCode,
			previewOpenAIResponse(responseBody),
		)
	}

	var payload openAIEmbeddingsResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI embeddings response: %w", err)
	}

	if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
		return nil, fmt.Errorf("OpenAI embeddings returned error: %s", payload.Error.Message)
	}

	if len(payload.Data) != len(inputs) {
		return nil, fmt.Errorf(
			"OpenAI embeddings returned %d vectors for %d inputs",
			len(payload.Data),
			len(inputs),
		)
	}

	sort.Slice(payload.Data, func(i, j int) bool {
		return payload.Data[i].Index < payload.Data[j].Index
	})

	embeddings := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if len(item.Embedding) != openAITextEmbeddingDim {
			return nil, fmt.Errorf(
				"OpenAI embedding dimension mismatch: expected %d, got %d",
				openAITextEmbeddingDim,
				len(item.Embedding),
			)
		}
		embeddings = append(embeddings, formatEmbeddingVector(item.Embedding))
	}

	return embeddings, nil
}

func formatEmbeddingVector(values []float64) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%g", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

func previewOpenAIResponse(body []byte) string {
	text := string(bytes.ToValidUTF8(body, []byte{}))
	text = strings.TrimSpace(text)
	if text == "" {
		return "<empty>"
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > openAIEmbeddingResponsePreviewLimit {
		return text[:openAIEmbeddingResponsePreviewLimit] + "..."
	}
	return text
}
