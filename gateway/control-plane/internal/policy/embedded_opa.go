package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/rego"
)

type EmbeddedOPA struct {
	query *rego.PreparedEvalQuery
}

func NewEmbeddedOPA(policyDir string) (*EmbeddedOPA, error) {
	if policyDir == "" {
		policyDir = "policies"
	}

	regoFiles, err := findRegoFiles(policyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find rego files: %w", err)
	}

	opts := []func(*rego.Rego){
		rego.Query("data.agentid.authz"),
	}

	for _, f := range regoFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read rego file %s: %w", f, err)
		}
		opts = append(opts, rego.Module(filepath.Base(f), string(content)))
	}

	query, err := rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare rego query: %w", err)
	}

	return &EmbeddedOPA{query: &query}, nil
}

func (e *EmbeddedOPA) Evaluate(ctx context.Context, input PolicyInput) (*Decision, error) {
	inputMap, err := inputToMap(input)
	if err != nil {
		return nil, err
	}

	rs, err := e.query.Eval(ctx, rego.EvalInput(inputMap))
	if err != nil {
		return nil, fmt.Errorf("rego evaluation failed: %w", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return &Decision{
			Allowed:      false,
			RequiresHITL: true,
			Reason:       "no policy result",
			TrustDelta:   -0.05,
		}, nil
	}

	result, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return &Decision{
			Allowed:      false,
			RequiresHITL: true,
			Reason:       "unexpected policy result type",
			TrustDelta:   -0.05,
		}, nil
	}

	decision := &Decision{
		Allowed:            boolValue(result["allow"]),
		RequiresHITL:       boolValue(result["requires_hitl"]),
		RequiresEscalation: boolValue(result["requires_escalation"]),
		Reason:             stringValue(result["reason"]),
		TrustDelta:         floatValue(result["trust_delta"]),
		RiskLevel:          stringValue(result["risk_level"]),
	}

	escalationLevel, ok := result["requires_escalation"]
	if ok {
		if boolValue(escalationLevel) {
			decision.EscalationLevel = 2
			decision.RequiredApprovals = 2
		}
	}
	if decision.RequiresHITL && !decision.RequiresEscalation {
		decision.EscalationLevel = 1
		decision.RequiredApprovals = 1
	}

	return decision, nil
}

func boolValue(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func floatValue(v interface{}) float64 {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0
		}
		return f
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func inputToMap(input PolicyInput) (map[string]interface{}, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func findRegoFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".rego" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}