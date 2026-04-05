package graphs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smallnest/langgraphgo/graph"
)

const conditionNextKeyPrefix = "__condition_next:"

type conditionConfig struct {
	SourceKey     string `json:"source_key"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
	CaseSensitive bool   `json:"case_sensitive"`
	TrueTarget    string `json:"true_target"`
	FalseTarget   string `json:"false_target"`
}

type ConditionRoutingResult struct {
	Node              *NodeToAdd
	ConditionalEdgeFn func(ctx context.Context, state map[string]any) string
	Config            conditionConfig
}

func parseConditionConfig(snapshotNode *SnapshotNode) (conditionConfig, error) {
	var cfg conditionConfig
	if err := json.Unmarshal([]byte(snapshotNode.Node.Config), &cfg); err != nil {
		return conditionConfig{}, fmt.Errorf("condition node %q: invalid config json: %w", snapshotNode.Node.NodeKey, err)
	}

	cfg.SourceKey = strings.TrimSpace(cfg.SourceKey)
	cfg.Operator = strings.ToLower(strings.TrimSpace(cfg.Operator))
	cfg.Value = strings.TrimSpace(cfg.Value)
	cfg.TrueTarget = strings.TrimSpace(cfg.TrueTarget)
	cfg.FalseTarget = strings.TrimSpace(cfg.FalseTarget)

	if cfg.SourceKey == "" {
		return conditionConfig{}, fmt.Errorf("condition node %q: config must specify source_key", snapshotNode.Node.NodeKey)
	}
	switch cfg.Operator {
	case "contains", "equals":
	default:
		return conditionConfig{}, fmt.Errorf("condition node %q: operator must be contains or equals", snapshotNode.Node.NodeKey)
	}
	if cfg.TrueTarget == "" {
		return conditionConfig{}, fmt.Errorf("condition node %q: config must specify true_target", snapshotNode.Node.NodeKey)
	}
	if cfg.FalseTarget == "" {
		return conditionConfig{}, fmt.Errorf("condition node %q: config must specify false_target", snapshotNode.Node.NodeKey)
	}

	return cfg, nil
}

func BuildConditionNode(snapshotNode *SnapshotNode) (*ConditionRoutingResult, error) {
	cfg, err := parseConditionConfig(snapshotNode)
	if err != nil {
		return nil, err
	}

	nextKey := conditionNextStateKey(snapshotNode.Node.NodeKey)

	return &ConditionRoutingResult{
		Node: &NodeToAdd{
			Name:        snapshotNode.Node.NodeKey,
			Description: snapshotNode.Node.NodeKey,
			Fn: func(ctx context.Context, state map[string]any) (map[string]any, error) {
				source := readConditionSourceValue(state[cfg.SourceKey])
				matched := evaluateCondition(source, cfg)
				nextTarget := cfg.FalseTarget
				if matched {
					nextTarget = cfg.TrueTarget
				}

				delta := map[string]any{
					nextKey: nextTarget,
				}
				if snapshotNode.Node.OutputKey != nil {
					delta[*snapshotNode.Node.OutputKey] = matched
				}
				return delta, nil
			},
		},
		ConditionalEdgeFn: func(ctx context.Context, state map[string]any) string {
			if next, ok := state[nextKey].(string); ok {
				next = strings.TrimSpace(next)
				if next != "" {
					if next == graph.END {
						return graph.END
					}
					return next
				}
			}
			if cfg.FalseTarget == graph.END {
				return graph.END
			}
			return cfg.FalseTarget
		},
		Config: cfg,
	}, nil
}

func conditionNextStateKey(nodeKey string) string {
	return conditionNextKeyPrefix + nodeKey
}

func evaluateCondition(source string, cfg conditionConfig) bool {
	left := source
	right := cfg.Value
	if !cfg.CaseSensitive {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}

	switch cfg.Operator {
	case "contains":
		return strings.Contains(left, right)
	case "equals":
		return left == right
	default:
		return false
	}
}

func readConditionSourceValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
