package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestExtractValueByPath_RootScalar(t *testing.T) {
	got, err := extractValueByPath("https://example.com/output.png", "$")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com/output.png" {
		t.Fatalf("unexpected root value: %v", got)
	}
}

func TestExtractStringByPath_ObjectField(t *testing.T) {
	got, err := extractStringByPath(map[string]any{
		"img_out": "https://example.com/output.png",
	}, "img_out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com/output.png" {
		t.Fatalf("unexpected field value: %v", got)
	}
}

func TestResolveSegmentationResult_DownloadsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"segments":[{"label":"car"}]}`))
	}))
	defer server.Close()

	got, err := resolveSegmentationResult(context.Background(), map[string]any{
		"json_out": server.URL,
	}, segmentationModelConfig{
		ResultPath:   "json_out",
		ResultSource: "url_json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"segments": []any{
			map[string]any{"label": "car"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("expected %s, got %s", wantJSON, gotJSON)
	}
}
