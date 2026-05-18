//go:build !pro

package license

func Load() Info {
	return Info{
		Tier:         TierCommunity,
		MaxAgents:    5,
		MaxResources: 10,
		Features: []string{
			FeatureDelegation,
		},
	}
}

func Validate(_ string) error {
	return nil
}
