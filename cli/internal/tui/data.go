package tui

// Agent data structure
type Agent struct {
	Name   string
	DID    string
	Trust  int
	Status string
}

// Policy data structure
type Policy struct {
	Action   string
	Decision string
	Reason   string
}

// AuditLog data structure
type AuditLog struct {
	Timestamp string
	Agent     string
	Event     string
	Decision  string
	Hash      string
}

// HITLRequest - Human in the Loop request
type HITLRequest struct {
	ID         string
	AgentID    string
	Action     string
	ResourceID string
	RiskLevel  string
	Status     string
	CreatedAt  string
}

// AirportAgent - agent visible at the Airport
type AirportAgent struct {
	AgentID    string
	Name       string
	Status     string
	TrustScore float64
	Tags       string
}

// Skill - agent skill from catalog
type Skill struct {
	ID          string
	Name        string
	Category    string
	RiskLevel   string
	TrustMin    string
	Proficiency string
}

// ServiceStatus - system service health
type ServiceStatus struct {
	Name   string
	Status string
	Uptime string
	Note   string
}

// Demo data
var DemoAgents = []Agent{
	{Name: "hermes-ops", DID: "did:eyevesa:agent:7F3A", Trust: 92, Status: "VERIFIED"},
	{Name: "researcher", DID: "did:eyevesa:agent:A91C", Trust: 87, Status: "VERIFIED"},
	{Name: "deploy-bot", DID: "did:eyevesa:agent:09BE", Trust: 61, Status: "RESTRICTED"},
	{Name: "unknown", DID: "did:eyevesa:agent:FFFF", Trust: 12, Status: "BLOCKED"},
}

var DemoPolicies = []Policy{
	{Action: "log_search", Decision: "ALLOW", Reason: "low-risk"},
	{Action: "k8s_deploy", Decision: "HUMAN_APPROVAL", Reason: "production-change"},
	{Action: "delete_bucket", Decision: "DENY", Reason: "destructive-action"},
	{Action: "db_migration", Decision: "HUMAN_APPROVAL", Reason: "schema-risk"},
}

var DemoAuditLogs = []AuditLog{
	{Timestamp: "12:41:22", Agent: "hermes-ops", Event: "requested k8s_deploy", Decision: "human-in-the-loop", Hash: "0x9fa...21c"},
	{Timestamp: "12:41:23", Agent: "hermes-ops", Event: "policy matched: production_change", Decision: "human-in-the-loop", Hash: "0x9fa...21c"},
	{Timestamp: "12:41:24", Agent: "hermes-ops", Event: "decision: human-in-the-loop", Decision: "human-in-the-loop", Hash: "0x9fa...21c"},
	{Timestamp: "12:42:01", Agent: "researcher", Event: "requested log_search", Decision: "ALLOW", Hash: "0x3ab...88e"},
	{Timestamp: "12:43:10", Agent: "deploy-bot", Event: "requested delete_bucket", Decision: "DENY", Hash: "0xe2b...cc1"},
}

var DemoHITL = []HITLRequest{
	{ID: "hitl-001", AgentID: "hermes-ops", Action: "k8s_deploy", ResourceID: "prod-cluster", RiskLevel: "HIGH", Status: "PENDING", CreatedAt: "12:41:22"},
	{ID: "hitl-002", AgentID: "deploy-bot", Action: "db_migration", ResourceID: "postgres-prod", RiskLevel: "HIGH", Status: "PENDING", CreatedAt: "12:43:00"},
}

var DemoAirport = []AirportAgent{
	{AgentID: "agent-7f3a", Name: "hermes-ops", Status: "online", TrustScore: 0.92, Tags: "ops,deploy"},
	{AgentID: "agent-a91c", Name: "researcher", Status: "online", TrustScore: 0.87, Tags: "research,read"},
	{AgentID: "agent-09be", Name: "deploy-bot", Status: "busy", TrustScore: 0.61, Tags: "deploy"},
}

var DemoSkills = []Skill{
	{ID: "skill-001", Name: "kubernetes", Category: "deployment", RiskLevel: "high", TrustMin: "0.70", Proficiency: "3"},
	{ID: "skill-002", Name: "log_search", Category: "observability", RiskLevel: "low", TrustMin: "0.30", Proficiency: "1"},
	{ID: "skill-003", Name: "database", Category: "data", RiskLevel: "critical", TrustMin: "0.85", Proficiency: "2"},
	{ID: "skill-004", Name: "git_push", Category: "source-control", RiskLevel: "medium", TrustMin: "0.50", Proficiency: "2"},
}

var DemoSystemStatus = []ServiceStatus{
	{Name: "gateway-control", Status: "UP", Uptime: "12h 33m", Note: "HTTP+gRPC API"},
	{Name: "gateway-core", Status: "UP", Uptime: "12h 33m", Note: "Rust MCP proxy"},
	{Name: "postgres", Status: "UP", Uptime: "12h 40m", Note: "pgvector enabled"},
	{Name: "opa-engine", Status: "UP", Uptime: "12h 33m", Note: "Rego authz"},
	{Name: "audit-ledger", Status: "UP", Uptime: "12h 33m", Note: "Ed25519 signed"},
}

// Boot sequence logs
var BootSequenceMessages = []string{
	"initializing eyeVesa gateway...",
	"loading agent identity registry...",
	"syncing policy engine...",
	"opening audit ledger...",
	"connecting airport mesh...",
	"loading federation peers...",
	"gateway status: ACTIVE",
}

// TrustBundle - SPIRE/federation trust bundle
type TrustBundle struct {
	TrustDomain string
	Type        string
	Source      string
	Federated   string
	Status      string
}

// APIKeyEntry - API key record
type APIKeyEntry struct {
	ID        string
	Name      string
	TenantID  string
	Status    string
	CreatedAt string
}

// SecurityEvent - CI/CD security scan
type SecurityEvent struct {
	Workflow   string
	Status     string
	Conclusion string
	Branch     string
	RunAt      string
}

var DemoTrustBundles = []TrustBundle{
	{TrustDomain: "eyevesa.local", Type: "spiffe_x509", Source: "static", Federated: "NO", Status: "VERIFIED"},
	{TrustDomain: "partner.acme", Type: "spiffe_x509", Source: "web", Federated: "YES", Status: "VERIFIED"},
	{TrustDomain: "cloud.prod", Type: "jwt", Source: "static", Federated: "YES", Status: "PENDING"},
}

var DemoAPIKeys = []APIKeyEntry{
	{ID: "key-a1b2", Name: "ci-deploy", TenantID: "org:acme", Status: "ACTIVE", CreatedAt: "2026-05-01"},
	{ID: "key-c3d4", Name: "monitoring", TenantID: "org:acme", Status: "ACTIVE", CreatedAt: "2026-05-10"},
	{ID: "key-e5f6", Name: "legacy-bot", TenantID: "", Status: "REVOKED", CreatedAt: "2026-04-01"},
}

var DemoSecurityEvents = []SecurityEvent{
	{Workflow: "Security Phase 1", Status: "completed", Conclusion: "success", Branch: "main", RunAt: "12:00"},
	{Workflow: "Container Scan Gate", Status: "completed", Conclusion: "success", Branch: "main", RunAt: "12:05"},
	{Workflow: "Post Deploy Smoke", Status: "completed", Conclusion: "success", Branch: "main", RunAt: "12:10"},
	{Workflow: "Alerting & Routing", Status: "completed", Conclusion: "failure", Branch: "main", RunAt: "12:15"},
}
