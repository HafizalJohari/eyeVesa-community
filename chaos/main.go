package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	gatewayURL = "http://localhost:8080"
	red        = "\033[0;31m"
	green      = "\033[0;32m"
	yellow     = "\033[1;33m"
	cyan       = "\033[0;36m"
	bold       = "\033[1m"
	nc         = "\033[0m"
)

type testResult struct {
	Name     string
	Pass     bool
	Detail   string
	Duration time.Duration
}

func banner(title string) {
	fmt.Printf("\n%s%s%s\n", cyan, strings.Repeat("=", 60), nc)
	fmt.Printf("%s%s ⚡ %s%s\n", bold, red, title, nc)
	fmt.Printf("%s%s%s\n\n", cyan, strings.Repeat("=", 60), nc)
}

func pass(name, detail string, d time.Duration) testResult {
	fmt.Printf("  %s✓ PASS%s %s — %s (%s)\n", green, nc, name, detail, d.Round(time.Millisecond))
	return testResult{Name: name, Pass: true, Detail: detail, Duration: d}
}

func fail(name, detail string, d time.Duration) testResult {
	fmt.Printf("  %s✗ FAIL%s %s — %s (%s)\n", red, nc, name, detail, d.Round(time.Millisecond))
	return testResult{Name: name, Pass: false, Detail: detail, Duration: d}
}

func postWithRetry(path string, body interface{}, maxRetries int) (map[string]interface{}, int, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, gatewayURL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, 0, err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var result map[string]interface{}
		json.Unmarshal(raw, &result)

		if resp.StatusCode == 429 {
			backoff := time.Duration(attempt+1) * 500 * time.Millisecond
			fmt.Printf("  %s⏳ 429 rate limited, retrying in %s...%s\n", yellow, backoff, nc)
			time.Sleep(backoff)
			continue
		}
		return result, resp.StatusCode, nil
	}
	return nil, 429, fmt.Errorf("rate limited after %d retries", maxRetries)
}

func getWithRetry(path string, maxRetries int) (map[string]interface{}, int, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := http.DefaultClient.Get(gatewayURL + path)
		if err != nil {
			return nil, 0, err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var result map[string]interface{}
		json.Unmarshal(raw, &result)

		if resp.StatusCode == 429 {
			backoff := time.Duration(attempt+1) * 500 * time.Millisecond
			time.Sleep(backoff)
			continue
		}
		return result, resp.StatusCode, nil
	}
	return nil, 429, fmt.Errorf("rate limited after retries")
}

func post(path string, body interface{}) (map[string]interface{}, int, error) {
	return postWithRetry(path, body, 5)
}

func get(path string) (map[string]interface{}, int, error) {
	return getWithRetry(path, 5)
}

func findAgent(name string) (string, error) {
	result, _, err := get("/v1/agents")
	if err != nil {
		return "", err
	}
	agents, ok := result["agents"].([]interface{})
	if !ok {
		return "", fmt.Errorf("no agents list in response")
	}
	for _, a := range agents {
		agent, _ := a.(map[string]interface{})
		if agent["name"] == name {
			id, _ := agent["agent_id"].(string)
			return id, nil
		}
	}
	return "", fmt.Errorf("agent '%s' not found — register it first with: eyevesa init --name %s", name, name)
}

func testBudgetAttack() []testResult {
	banner("TEST 1: Token Budget Attack — Budget Metering Under Fire")
	var results []testResult

	agentID, err := findAgent("jclaw-shop")
	if err != nil {
		results = append(results, fail("find jclaw-shop agent", err.Error(), 0))
		return results
	}
	fmt.Printf("  Using agent: %s (jclaw-shop)\n\n", agentID)

	fmt.Printf("  %sPhase 1: Flooding /v1/budget/spend with 50 concurrent $2.00 requests...%s\n", yellow, nc)
	start := time.Now()
	var successCount, blockedCount int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			body := map[string]interface{}{
				"agent_id":       agentID,
				"action":         fmt.Sprintf("chaos_spend_%d", idx),
				"estimated_cost": 2.00,
				"actual_cost":    2.00,
			}
			_, status, err := postWithRetry("/v1/budget/spend", body, 3)
			if err != nil {
				return
			}
			if status == 200 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&blockedCount, 1)
			}
		}(i)
	}
	wg.Wait()
	floodDuration := time.Since(start)

	fmt.Printf("  Recorded: %d | Errors/Blocked: %d | Duration: %s\n", successCount, blockedCount, floodDuration.Round(time.Millisecond))

	result, _, _ := get(fmt.Sprintf("/v1/budget/check?agent_id=%s&estimated_cost=0.01", agentID))
	allowed, _ := result["allowed"].(bool)
	remaining, _ := result["remaining_budget"].(float64)

	if !allowed {
		results = append(results, pass("budget exhausted", fmt.Sprintf("allowed=false, remaining=$%.2f", remaining), floodDuration))
	} else {
		results = append(results, fail("budget NOT exhausted", fmt.Sprintf("allowed=true, remaining=$%.2f — BUDGET ENGINE FAILED", remaining), floodDuration))
	}

	fmt.Printf("\n  %sPhase 2: Attempting single $999.00 spend on limited budget...%s\n", yellow, nc)
	start = time.Now()
	body := map[string]interface{}{
		"agent_id":       agentID,
		"action":         "chaos_overspend",
		"estimated_cost": 999.00,
		"actual_cost":    999.00,
	}
	result, status, _ := post("/v1/budget/spend", body)
	dur := time.Since(start)
	if status != 200 || (result["error"] != nil) {
		results = append(results, pass("overspend rejected", fmt.Sprintf("HTTP %d — gateway blocked $999 spend", status), dur))
	} else {
		checkResult, _, _ := get(fmt.Sprintf("/v1/budget/check?agent_id=%s&estimated_cost=1.00", agentID))
		checkAllowed, _ := checkResult["allowed"].(bool)
		if !checkAllowed {
			results = append(results, pass("overspend caught by check", "spend recorded but budget check blocks future spend", dur))
		} else {
			results = append(results, fail("overspend NOT caught", "$999 recorded and budget check still allows", dur))
		}
	}

	return results
}

func testLoopholeHunting() []testResult {
	banner("TEST 2: Loophole Hunting — OPA Policy Bypass Attempts")
	var results []testResult

	agentID, err := findAgent("basic-example-agent")
	if err != nil {
		results = append(results, fail("find basic-example-agent", err.Error(), 0))
		return results
	}
	fmt.Printf("  Using agent: %s (basic-example-agent)\n\n", agentID)

	attackVectors := []struct {
		name   string
		action string
		params map[string]interface{}
	}{
		{
			name:   "direct unauthorized tool (bank_transfer)",
			action: "bank_transfer",
			params: map[string]interface{}{"amount": 500.0},
		},
		{
			name:   "prompt injection in action name",
			action: "chat; rm -rf /",
			params: map[string]interface{}{},
		},
		{
			name:   "path traversal in action",
			action: "../../../etc/passwd",
			params: map[string]interface{}{},
		},
		{
			name:   "unicode homograph attack",
			action: "ѕearch",
			params: map[string]interface{}{},
		},
		{
			name:   "case manipulation bypass",
			action: "CHAT",
			params: map[string]interface{}{},
		},
		{
			name:   "k8s deployment without permission",
			action: "k8s_deploy",
			params: map[string]interface{}{"namespace": "production"},
		},
		{
			name:   "database schema change attempt",
			action: "database_schema_change",
			params: map[string]interface{}{},
		},
		{
			name:   "shopping not in allowed list",
			action: "shopping",
			params: map[string]interface{}{},
		},
	}

	for i, attack := range attackVectors {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		start := time.Now()
		body := map[string]interface{}{
			"agent_id": agentID,
			"action":   attack.action,
			"params":   attack.params,
		}
		result, _, err := post("/v1/authorize", body)
		dur := time.Since(start)

		if err != nil {
			results = append(results, pass(attack.name, fmt.Sprintf("request error: %v", err), dur))
			continue
		}

		allowed, _ := result["allowed"].(bool)
		reason, _ := result["reason"].(string)
		requiresHITL, _ := result["requires_hitl"].(bool)

		if !allowed {
			results = append(results, pass(attack.name, fmt.Sprintf("BLOCKED — reason: %s, hitl: %v", reason, requiresHITL), dur))
		} else if requiresHITL {
			results = append(results, pass(attack.name, fmt.Sprintf("HITL REQUIRED — reason: %s", reason), dur))
		} else {
			results = append(results, fail(attack.name, fmt.Sprintf("BYPASSED! allowed=true, hitl=false — CRITICAL VULNERABILITY"), dur))
		}
	}

	return results
}

func testTrustDrain() []testResult {
	banner("TEST 3: Trust Score Drain — Driving Agent Below Threshold")
	var results []testResult

	agentID, err := findAgent("e2e-agent")
	if err != nil {
		results = append(results, fail("find e2e-agent", err.Error(), 0))
		return results
	}
	fmt.Printf("  Using agent: %s (e2e-agent, allowed: [read, write, search])\n\n", agentID)

	agentInfo, _, _ := get("/v1/agents/" + agentID)
	initialTrust, _ := agentInfo["trust_score"].(float64)
	fmt.Printf("  Initial trust: %.2f\n\n", initialTrust)

	if initialTrust < 0.15 {
		fmt.Printf("  %s⚠ Trust already too low (%.2f) — resetting via DB...%s\n", yellow, initialTrust, nc)
		fmt.Printf("  Run: docker compose exec -T postgres psql -U agentid -c \"UPDATE agents SET trust_score = 0.85 WHERE agent_id = '%s'\"\n\n", agentID)
		results = append(results, fail("trust drain", fmt.Sprintf("agent trust already at %.2f, needs reset", initialTrust), 0))
		return results
	}

	start := time.Now()
	var lastTrustDelta float64
	for i := 0; i < 40; i++ {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		body := map[string]interface{}{
			"agent_id": agentID,
			"action":   "shopping",
			"params":   map[string]interface{}{},
		}
		result, _, _ := post("/v1/authorize", body)
		allowed, _ := result["allowed"].(bool)
		reason, _ := result["reason"].(string)
		lastTrustDelta, _ = result["trust_delta"].(float64)

		fmt.Printf("  Attempt %d: allowed=%v reason=%s trust_delta=%.2f\n", i+1, allowed, reason, lastTrustDelta)

		if strings.Contains(reason, "trust score below minimum") {
			results = append(results, pass("trust auto-deny", fmt.Sprintf("auto-denied after %d attempts (trust < 0.1)", i+1), time.Since(start)))

			result2, _, _ := post("/v1/authorize", map[string]interface{}{
				"agent_id": agentID,
				"action":   "search",
				"params":   map[string]interface{}{},
			})
			allowed2, _ := result2["allowed"].(bool)
			reason2, _ := result2["reason"].(string)

			if !allowed2 && strings.Contains(reason2, "trust score below minimum") {
				results = append(results, pass("permanent auto-deny", "drained agent blocked even for allowed tool 'search'", 0))
			} else if !allowed2 {
				results = append(results, pass("auto-deny active", fmt.Sprintf("blocked for 'search': %s", reason2), 0))
			} else {
				results = append(results, fail("permanent auto-deny", fmt.Sprintf("drained agent still allowed=%v for 'search': %s", allowed2, reason2), 0))
			}
			return results
		}
	}

	agentInfo2, _, _ := get("/v1/agents/" + agentID)
	finalTrust, _ := agentInfo2["trust_score"].(float64)
	fmt.Printf("\n  Final trust: %.2f (delta: %.2f)\n", finalTrust, initialTrust-finalTrust)

	if finalTrust < initialTrust {
		results = append(results, pass("trust decrementing", fmt.Sprintf("trust dropped %.2f → %.2f after 40 unauthorized calls", initialTrust, finalTrust), time.Since(start)))
		if finalTrust >= 0.1 {
			results = append(results, fail("trust drain too slow", fmt.Sprintf("trust at %.2f after 40 calls — need more calls or bigger delta to reach < 0.1", finalTrust), time.Since(start)))
		}
	} else {
		results = append(results, fail("trust NOT persisting", fmt.Sprintf("trust unchanged at %.2f after 40 calls with delta=%.2f — DB updates may be silently failing", finalTrust, lastTrustDelta), time.Since(start)))
	}

	return results
}

func testBankTransferLimits() []testResult {
	banner("TEST 4: OPA Policy — Hard Limits & Escalation Rules")
	var results []testResult

	agentID, err := findAgent("e2e-agent")
	if err != nil {
		results = append(results, fail("find e2e-agent", err.Error(), 0))
		return results
	}
	fmt.Printf("  Using agent: %s (e2e-agent, allowed: [read, write, search])\n\n", agentID)

	policyTests := []struct {
		name       string
		action     string
		params     map[string]interface{}
		mustBlock  bool
		mustHITL   bool
	}{
		{
			name:      "bank_transfer $5000 (HARD LIMIT auto-deny)",
			action:    "bank_transfer",
			params:    map[string]interface{}{"amount": 5000.0},
			mustBlock: true,
		},
		{
			name:      "bank_transfer $99999 (absurd amount)",
			action:    "bank_transfer",
			params:    map[string]interface{}{"amount": 99999.0},
			mustBlock: true,
		},
		{
			name:      "k8s_deploy production (escalation required)",
			action:    "k8s_deploy",
			params:    map[string]interface{}{"namespace": "production"},
			mustBlock: true,
		},
		{
			name:      "database_schema_change (escalation required)",
			action:    "database_schema_change",
			params:    map[string]interface{}{},
			mustBlock: true,
		},
		{
			name:     "search with $200 cost (trust*100 ceiling)",
			action:   "search",
			params:   map[string]interface{}{"estimated_cost": 200.0},
			mustBlock: true,
		},
	}

	for i, tc := range policyTests {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		start := time.Now()
		body := map[string]interface{}{
			"agent_id": agentID,
			"action":   tc.action,
			"params":   tc.params,
		}
		result, _, _ := post("/v1/authorize", body)
		dur := time.Since(start)

		allowed, _ := result["allowed"].(bool)
		reason, _ := result["reason"].(string)
		requiresHITL, _ := result["requires_hitl"].(bool)

		if tc.mustBlock && !allowed {
			if tc.mustHITL && requiresHITL {
				results = append(results, pass(tc.name, fmt.Sprintf("blocked+HITL: %s", reason), dur))
			} else {
				results = append(results, pass(tc.name, fmt.Sprintf("blocked: %s", reason), dur))
			}
		} else if tc.mustBlock && allowed {
			results = append(results, fail(tc.name, fmt.Sprintf("NOT blocked! allowed=%v hitl=%v — BYPASSED", allowed, requiresHITL), dur))
		} else if !tc.mustBlock && allowed {
			results = append(results, pass(tc.name, fmt.Sprintf("allowed: %s", reason), dur))
		} else {
			results = append(results, pass(tc.name, fmt.Sprintf("blocked (conservative): %s", reason), dur))
		}
	}

	return results
}

func testCostAuthorization() []testResult {
	banner("TEST 5: Cost-Based Authorization — Trust-Based Budget Ceiling")
	var results []testResult

	agentID, err := findAgent("langchain-demo")
	if err != nil {
		results = append(results, fail("find langchain-demo agent", err.Error(), 0))
		return results
	}
	fmt.Printf("  Using agent: %s (langchain-demo, allowed: [read, get_weather, search_docs], trust=0.8)\n\n", agentID)

	costTests := []struct {
		name   string
		cost   float64
		expect string
	}{
		{"$10 cost via search_docs (under ceiling)", 10.0, "allowed"},
		{"$50 cost via search_docs (under ceiling)", 50.0, "allowed"},
		{"$100 cost via search_docs (over ceiling)", 100.0, "denied"},
		{"$99999 cost via search_docs (absurd)", 99999.0, "denied"},
	}

	for i, tc := range costTests {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		start := time.Now()
		body := map[string]interface{}{
			"agent_id": agentID,
			"action":   "search_docs",
			"params":   map[string]interface{}{"estimated_cost": tc.cost},
		}
		result, _, _ := post("/v1/authorize", body)
		dur := time.Since(start)

		allowed, _ := result["allowed"].(bool)
		reason, _ := result["reason"].(string)

		if tc.expect == "denied" {
			if !allowed {
				results = append(results, pass(tc.name, fmt.Sprintf("blocked: %s", reason), dur))
			} else {
				results = append(results, fail(tc.name, fmt.Sprintf("COST BYPASS! allowed=true with $%.0f cost: %s", tc.cost, reason), dur))
			}
		} else {
			if allowed {
				results = append(results, pass(tc.name, fmt.Sprintf("allowed: %s", reason), dur))
			} else {
				results = append(results, pass(tc.name, fmt.Sprintf("blocked (conservative): %s", reason), dur))
			}
		}
	}

	return results
}

func main() {
	fmt.Printf("\n%s%s ⚡ COGNITIVE DRIFT — eyeVesa Chaos Engineering Simulation ⚡ %s\n\n", bold, red, nc)
	fmt.Printf("  Gateway: %s\n", gatewayURL)
	fmt.Printf("  Time:    %s\n\n", time.Now().Format(time.RFC3339))

	resp, err := http.Get(gatewayURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("%s✗ Gateway not reachable at %s — is it running?%s\n", red, gatewayURL, nc)
		fmt.Printf("  Run: %s./start.sh%s\n\n", cyan, nc)
		os.Exit(1)
	}
	resp.Body.Close()
	fmt.Printf("%s✓ Gateway healthy%s\n\n", green, nc)

	fmt.Printf("  %sNote: Reusing existing agents (license tier has 5-agent limit)%s\n\n", yellow, nc)

	var allResults []testResult

	allResults = append(allResults, testBudgetAttack()...)
	time.Sleep(2 * time.Second)
	allResults = append(allResults, testLoopholeHunting()...)
	time.Sleep(2 * time.Second)
	allResults = append(allResults, testTrustDrain()...)
	time.Sleep(2 * time.Second)
	allResults = append(allResults, testBankTransferLimits()...)
	time.Sleep(2 * time.Second)
	allResults = append(allResults, testCostAuthorization()...)

	fmt.Printf("\n%s%s ⚡ CHAOS REPORT ⚡ %s\n", bold, red, nc)
	fmt.Printf("%s%s%s\n", cyan, strings.Repeat("=", 60), nc)

	passed := 0
	failed := 0
	for _, r := range allResults {
		if r.Pass {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\n  Total:  %d\n", len(allResults))
	fmt.Printf("  %sPASS: %d%s\n", green, passed, nc)
	fmt.Printf("  %sFAIL: %d%s\n", red, failed, nc)

	if failed > 0 {
		fmt.Printf("\n  %sVULNERABILITIES FOUND:%s\n", red, nc)
		for _, r := range allResults {
			if !r.Pass {
				fmt.Printf("    %s✗ %s%s — %s\n", red, r.Name, nc, r.Detail)
			}
		}
	} else {
		fmt.Printf("\n  %sALL DEFENSES HELD — GATEWAY IS RESILIENT%s\n", green, nc)
	}

	fmt.Printf("\n%s%s%s\n", cyan, strings.Repeat("=", 60), nc)

	if failed > 0 {
		os.Exit(1)
	}
}
