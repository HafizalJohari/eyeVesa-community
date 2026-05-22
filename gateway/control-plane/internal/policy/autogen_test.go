package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAutogenRegoSkipsNeverEventsBeforeCall(t *testing.T) {
	if !isNeverEvent("schema.migrate") {
		t.Fatal("schema.migrate must be treated as a never-event")
	}
	if !isNeverEvent("cluster.modify.node") {
		t.Fatal("cluster.modify.* must be treated as a never-event")
	}
	if isNeverEvent("tool.scan") {
		t.Fatal("ordinary tool action should be eligible")
	}

	content := buildAutogenRego([]autogenRule{
		{AgentID: "agent-2", Action: "tool.scan", Count: 100},
		{AgentID: "agent-1", Action: "tool.read", Count: 100},
	})
	if !strings.Contains(content, "package agentid.authz") {
		t.Fatal("generated policy must use the production authz package")
	}
	if !strings.Contains(content, "tool_allowed {") {
		t.Fatal("generated policy should extend tool_allowed so HITL is cleared")
	}
	if !strings.Contains(content, `input.agent.id == "agent-1"`) {
		t.Fatal("generated policy should bind to a specific agent")
	}
	if !strings.Contains(content, `input.action.tool == "tool.read"`) {
		t.Fatal("generated policy should bind to a specific action")
	}
}

func TestValidateAndReplaceRejectsInvalidCandidateWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	authzPath := filepath.Join(dir, "authz.rego")
	if err := os.WriteFile(authzPath, []byte(`package agentid.authz

allow {
	input.action.tool == "read"
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	finalPath := filepath.Join(dir, autogenPolicyFile)
	if err := os.WriteFile(finalPath, []byte(`package agentid.authz

tool_allowed {
	input.action.tool == "existing"
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	worker := NewPolicyAutogenWorker(nil, dir)
	if err := worker.validateAndReplace("package agentid.authz\n\nallow {\n"); err == nil {
		t.Fatal("invalid generated policy should be rejected")
	}

	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"existing"`) {
		t.Fatal("invalid candidate overwrote the last valid autogen policy")
	}
}
