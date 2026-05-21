package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentid_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agentid_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	GRPCRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentid_grpc_requests_total",
		Help: "Total number of gRPC requests",
	}, []string{"method", "status"})

	AgentRegistrations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agentid_agent_registrations_total",
		Help: "Total number of agent registrations",
	})

	AuthorizationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentid_authorizations_total",
		Help: "Total number of authorization requests",
	}, []string{"decision"})

	HITLApprovals = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentid_hitl_approvals_total",
		Help: "Total number of HITL approval requests",
	}, []string{"action", "status"})

	DelegationsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agentid_delegations_total",
		Help: "Total number of delegation requests",
	})

	MPCToolCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentid_mcp_tool_calls_total",
		Help: "Total number of MCP tool calls",
	}, []string{"method"})

	RateLimitExceeded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agentid_rate_limit_exceeded_total",
		Help: "Total number of rate limit exceeded events",
	})

	ActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "agentid_active_connections",
		Help: "Number of active connections",
	})
)