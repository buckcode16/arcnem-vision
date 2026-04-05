package tools

import (
	"reflect"
	"testing"
)

func TestNormalizeOCRResultDeepseekMarkdown(t *testing.T) {
	result, err := normalizeOCRResult(map[string]any{
		"markdown": "# Title\n\nURGENT notice",
	}, ocrModelConfig{OCRAdapter: "deepseek_markdown"})
	if err != nil {
		t.Fatalf("normalizeOCRResult returned error: %v", err)
	}
	if result.Text != "# Title\n\nURGENT notice" {
		t.Fatalf("unexpected text: %q", result.Text)
	}
	if result.AvgConfidence != nil {
		t.Fatalf("expected nil confidence, got %v", *result.AvgConfidence)
	}
}

func TestNormalizeOCRResultDotsConfidence(t *testing.T) {
	result, err := normalizeOCRResult(map[string]any{
		"pages": []any{
			map[string]any{
				"text":       "Line one",
				"confidence": 0.84,
			},
			map[string]any{
				"text":       "Line two",
				"confidence": 91.0,
			},
		},
	}, ocrModelConfig{OCRAdapter: "dots_confidence"})
	if err != nil {
		t.Fatalf("normalizeOCRResult returned error: %v", err)
	}
	if result.Text != "Line one\n\nLine two" {
		t.Fatalf("unexpected text: %q", result.Text)
	}
	if result.AvgConfidence == nil {
		t.Fatal("expected confidence value")
	}
	if *result.AvgConfidence != 88 {
		t.Fatalf("expected avg confidence 88, got %d", *result.AvgConfidence)
	}
}

func TestExtractOCRTextFallsBackToArrays(t *testing.T) {
	text := extractOCRText([]any{
		map[string]any{"text": "First"},
		map[string]any{"content": "Second"},
	})
	if text != "First\n\nSecond" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestParseOCRModelConfigDefaults(t *testing.T) {
	cfg, err := parseOCRModelConfig(`{"ocr_adapter":"deepseek_markdown"}`)
	if err != nil {
		t.Fatalf("parseOCRModelConfig returned error: %v", err)
	}
	if cfg.InputImageField != "image" {
		t.Fatalf("expected default image field, got %q", cfg.InputImageField)
	}
	if !reflect.DeepEqual(cfg.InputDefaults, map[string]any{}) {
		t.Fatalf("expected empty input defaults, got %#v", cfg.InputDefaults)
	}
}
