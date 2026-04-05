package graphs

import (
	"context"
	"testing"

	dbmodels "github.com/arcnem-ai/arcnem-vision/models/db/gen/models"
)

func TestBuildConditionNodeContainsCaseInsensitive(t *testing.T) {
	outputKey := "matched"
	result, err := BuildConditionNode(&SnapshotNode{
		Node: &dbmodels.AgentGraphNode{
			NodeKey:   "route_keyword",
			NodeType:  "condition",
			OutputKey: &outputKey,
			Config:    `{"source_key":"ocr_text","operator":"contains","value":"urgent","true_target":"urgent_worker","false_target":"general_worker"}`,
		},
	})
	if err != nil {
		t.Fatalf("BuildConditionNode returned error: %v", err)
	}

	delta, err := result.Node.Fn(context.Background(), map[string]any{
		"ocr_text": "This label says URGENT action needed.",
	})
	if err != nil {
		t.Fatalf("condition node fn returned error: %v", err)
	}
	if got := delta[outputKey]; got != true {
		t.Fatalf("expected boolean output true, got %#v", got)
	}

	next := result.ConditionalEdgeFn(context.Background(), map[string]any{
		conditionNextStateKey("route_keyword"): delta[conditionNextStateKey("route_keyword")],
	})
	if next != "urgent_worker" {
		t.Fatalf("expected urgent_worker, got %q", next)
	}
}

func TestBuildConditionNodeEqualsCaseSensitiveFalseBranchToEnd(t *testing.T) {
	result, err := BuildConditionNode(&SnapshotNode{
		Node: &dbmodels.AgentGraphNode{
			NodeKey:  "route_exact",
			NodeType: "condition",
			Config:   `{"source_key":"ocr_text","operator":"equals","value":"coca cola","case_sensitive":true,"true_target":"brand_worker","false_target":"END"}`,
		},
	})
	if err != nil {
		t.Fatalf("BuildConditionNode returned error: %v", err)
	}

	delta, err := result.Node.Fn(context.Background(), map[string]any{
		"ocr_text": "Coca Cola",
	})
	if err != nil {
		t.Fatalf("condition node fn returned error: %v", err)
	}

	next := result.ConditionalEdgeFn(context.Background(), map[string]any{
		conditionNextStateKey("route_exact"): delta[conditionNextStateKey("route_exact")],
	})
	if next != "END" {
		t.Fatalf("expected END, got %q", next)
	}
}

func TestBuildConditionNodeMissingSourceFallsBackToFalseTarget(t *testing.T) {
	result, err := BuildConditionNode(&SnapshotNode{
		Node: &dbmodels.AgentGraphNode{
			NodeKey:  "route_missing",
			NodeType: "condition",
			Config:   `{"source_key":"ocr_text","operator":"contains","value":"urgent","true_target":"urgent_worker","false_target":"general_worker"}`,
		},
	})
	if err != nil {
		t.Fatalf("BuildConditionNode returned error: %v", err)
	}

	delta, err := result.Node.Fn(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("condition node fn returned error: %v", err)
	}

	next := result.ConditionalEdgeFn(context.Background(), map[string]any{
		conditionNextStateKey("route_missing"): delta[conditionNextStateKey("route_missing")],
	})
	if next != "general_worker" {
		t.Fatalf("expected general_worker, got %q", next)
	}
}
