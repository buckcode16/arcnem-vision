package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ocrModelConfig struct {
	InputImageField string         `json:"input_image_field"`
	InputDefaults   map[string]any `json:"input_defaults"`
	OCRAdapter      string         `json:"ocr_adapter"`
}

type normalizedOCRResult struct {
	Text          string
	AvgConfidence *int
	RawResult     any
}

func parseOCRModelConfig(raw string) (ocrModelConfig, error) {
	cfg := ocrModelConfig{}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "{}"
	}
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return ocrModelConfig{}, fmt.Errorf("invalid ocr model config: %w", err)
	}
	if strings.TrimSpace(cfg.InputImageField) == "" {
		cfg.InputImageField = "image"
	}
	if cfg.InputDefaults == nil {
		cfg.InputDefaults = map[string]any{}
	}
	cfg.OCRAdapter = strings.ToLower(strings.TrimSpace(cfg.OCRAdapter))
	if cfg.OCRAdapter == "" {
		return ocrModelConfig{}, fmt.Errorf("ocr model config must set ocr_adapter")
	}
	return cfg, nil
}

func normalizeOCRResult(rawOutput any, cfg ocrModelConfig) (normalizedOCRResult, error) {
	switch cfg.OCRAdapter {
	case "deepseek_markdown":
		text := strings.TrimSpace(extractOCRText(rawOutput))
		if text == "" {
			return normalizedOCRResult{}, fmt.Errorf("deepseek_markdown adapter produced empty text")
		}
		return normalizedOCRResult{
			Text:      text,
			RawResult: rawOutput,
		}, nil
	case "dots_confidence":
		text := strings.TrimSpace(extractOCRText(rawOutput))
		if text == "" {
			return normalizedOCRResult{}, fmt.Errorf("dots_confidence adapter produced empty text")
		}
		return normalizedOCRResult{
			Text:          text,
			AvgConfidence: averageOCRConfidence(rawOutput),
			RawResult:     rawOutput,
		}, nil
	default:
		return normalizedOCRResult{}, fmt.Errorf("unsupported ocr adapter %q", cfg.OCRAdapter)
	}
}

func extractOCRText(raw any) string {
	switch value := raw.(type) {
	case string:
		return cleanOCRText(value)
	case []byte:
		return cleanOCRText(string(value))
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			text := extractOCRText(item)
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return cleanOCRText(strings.Join(parts, "\n\n"))
	case map[string]any:
		for _, key := range []string{
			"markdown",
			"text",
			"full_text",
			"plain_text",
			"content",
			"transcription",
			"output_text",
			"result",
			"response",
			"pages",
			"predictions",
			"data",
		} {
			if next, ok := value[key]; ok {
				text := extractOCRText(next)
				if text != "" {
					return text
				}
			}
		}

		parts := make([]string, 0, len(value))
		for key, next := range value {
			if strings.EqualFold(strings.TrimSpace(key), "confidence") {
				continue
			}
			text := extractOCRText(next)
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return cleanOCRText(strings.Join(parts, "\n\n"))
	default:
		return ""
	}
}

func cleanOCRText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if lastBlank {
				continue
			}
			lastBlank = true
			cleaned = append(cleaned, "")
			continue
		}
		lastBlank = false
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func averageOCRConfidence(raw any) *int {
	values := make([]float64, 0, 8)
	collectOCRConfidence(raw, &values)
	if len(values) == 0 {
		return nil
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}
	average := sum / float64(len(values))
	rounded := int(math.Round(average))
	if rounded < 0 {
		rounded = 0
	}
	if rounded > 100 {
		rounded = 100
	}
	return &rounded
}

func collectOCRConfidence(raw any, out *[]float64) {
	switch value := raw.(type) {
	case map[string]any:
		for key, next := range value {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			switch normalizedKey {
			case "confidence", "avg_confidence", "average_confidence", "mean_confidence":
				if confidence, ok := normalizeConfidenceNumber(next); ok {
					*out = append(*out, confidence)
					continue
				}
			}
			collectOCRConfidence(next, out)
		}
	case []any:
		for _, next := range value {
			collectOCRConfidence(next, out)
		}
	}
}

func normalizeConfidenceNumber(value any) (float64, bool) {
	var raw float64
	switch typed := value.(type) {
	case float64:
		raw = typed
	case float32:
		raw = float64(typed)
	case int:
		raw = float64(typed)
	case int32:
		raw = float64(typed)
	case int64:
		raw = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		raw = parsed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		raw = parsed
	default:
		return 0, false
	}

	if raw >= 0 && raw <= 1 {
		raw *= 100
	}
	if raw < 0 || raw > 100 {
		return 0, false
	}
	return raw, true
}
