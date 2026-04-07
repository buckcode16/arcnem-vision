package load

import (
	"strings"
	"testing"
)

func TestLoadDocumentAndAgentGraphQueryUsesTemplateVersionID(t *testing.T) {
	query := loadDocumentAndAgentGraphQuery("LEFT JOIN agent_graphs ag ON ag.id = dev.agent_graph_id")

	if strings.Contains(query, "'agent_graph_template_version', ag.agent_graph_template_version") {
		t.Fatalf("query still references removed agent_graph_template_version column:\n%s", query)
	}

	if !strings.Contains(query, "'agent_graph_template_version_id', ag.agent_graph_template_version_id") {
		t.Fatalf("query does not project agent_graph_template_version_id:\n%s", query)
	}
}
