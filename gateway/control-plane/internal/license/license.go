package license

import "fmt"

type Tier string

const (
	TierCommunity Tier = "community"
	TierPro       Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

type Info struct {
	Tier      Tier
	MaxAgents int
	MaxResources int
	Features  []string
}

func (i Info) HasFeature(feature string) bool {
	for _, f := range i.Features {
		if f == feature {
			return true
		}
	}
	return false
}

func (i Info) String() string {
	return fmt.Sprintf("eyeVesa %s (max %d agents, %d resources)", i.Tier, i.MaxAgents, i.MaxResources)
}

const (
	// ── Community ──────────────────────────────────────────────────────────
	FeatureDelegation = "multi_level_delegation"
	FeatureFederation = "gateway_federation"

	// ── Pro ────────────────────────────────────────────────────────────────
	FeatureMultiTenant    = "multi_tenant"
	FeatureMultiLayerHITL = "multi_layer_hitl"
	FeatureSlackNotify    = "slack_notify"
	FeaturePagerDuty      = "pagerduty"
	FeatureSSO            = "sso"
	FeatureLLM            = "llm"
	FeatureAnomalyDetect  = "anomaly_detection"
	FeatureBudget         = "budget_enforcement"
	FeatureRateLimit      = "rate_limiting"
	FeatureKubernetes     = "kubernetes"
	FeaturePushNotify     = "push_notifications"

	// ── Enterprise ─────────────────────────────────────────────────────────
	FeatureSOC2            = "soc2"
	FeatureHIPAA           = "hipaa"
	FeatureManagedCloud    = "managed_cloud"
	FeatureDedicatedSupport = "dedicated_support"
	FeatureMultiRegion     = "multi_region"
	FeatureHSM             = "hsm_integration"
	FeatureCustomPolicies  = "custom_policies"
	FeatureCustomAdapters  = "custom_adapters"
)
