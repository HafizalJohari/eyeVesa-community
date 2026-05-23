//go:build !pro

package license

func Load() Info {
	return Info{
		Tier:         TierCommunity,
		MaxAgents:    100,
		MaxResources: 10,
		Features: []string{
			FeatureDelegation,
			FeatureFederation,
		},
	}
}

func Validate(_ string) error {
	return nil
}
