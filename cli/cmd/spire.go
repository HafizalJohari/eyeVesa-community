package cmd

import (
	"github.com/spf13/cobra"
)

var (
	spireTrustDomain  string
	spireBundleData   string
	spireBundleType   string
	spireSource       string
	spireEndpointURL  string
	spireIsFederated  bool
	spireFederatedOnly bool
	spireSpiffeID     string
	spireAgentID     string
	spireSelectors    []string
	spireParentID     string
	spireAutoRegister bool
)

var spireCmd = &cobra.Command{
	Use:   "spire",
	Short: "SPIRE identity and trust bundle management",
	Long:  "Manage SPIRE trust bundles, workload registrations, and federation for the AgentID Gateway.",
}

var spireBundleCreateCmd = &cobra.Command{
	Use:   "create-bundle",
	Short: "Create a trust bundle",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.CreateTrustBundle(spireTrustDomain, spireBundleData, spireBundleType, spireSource, spireEndpointURL, spireIsFederated)
		if err != nil {
			return err
		}
		printSuccess("Trust bundle created")
		printResult(result)
		return nil
	},
}

var spireBundleGetCmd = &cobra.Command{
	Use:   "get-bundle [trust-domain]",
	Short: "Get a trust bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetTrustBundle(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var spireBundleListCmd = &cobra.Command{
	Use:   "list-bundles",
	Short: "List trust bundles",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListTrustBundles(spireFederatedOnly)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var spireBundleUpdateCmd = &cobra.Command{
	Use:   "update-bundle [trust-domain]",
	Short: "Update a trust bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.UpdateTrustBundle(args[0], spireBundleData)
		if err != nil {
			return err
		}
		printSuccess("Trust bundle updated")
		printResult(result)
		return nil
	},
}

var spireBundleVerifyCmd = &cobra.Command{
	Use:   "verify-bundle [trust-domain]",
	Short: "Verify a trust bundle's cryptographic integrity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.VerifyTrustBundle(args[0])
		if err != nil {
			return err
		}
		printSuccess("Trust bundle verified")
		printResult(result)
		return nil
	},
}

var spireBundleDeleteCmd = &cobra.Command{
	Use:   "delete-bundle [trust-domain]",
	Short: "Delete a trust bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		_, err := client.DeleteTrustBundle(args[0])
		if err != nil {
			return err
		}
		printSuccess("Trust bundle deleted")
		return nil
	},
}

var spireBundleFetchCmd = &cobra.Command{
	Use:   "fetch-bundle",
	Short: "Fetch trust bundle from a remote SPIRE bundle endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.FetchBundleFromEndpoint(spireEndpointURL, spireTrustDomain, true, spireIsFederated)
		if err != nil {
			return err
		}
		printSuccess("Bundle fetched")
		printResult(result)
		return nil
	},
}

var spireWorkloadRegisterCmd = &cobra.Command{
	Use:   "register-workload",
	Short: "Register a SPIRE workload",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RegisterWorkload(spireSpiffeID, spireAgentID, spireTrustDomain, spireSelectors, spireParentID, spireAutoRegister)
		if err != nil {
			return err
		}
		printSuccess("Workload registered")
		printResult(result)
		return nil
	},
}

var spireWorkloadGetCmd = &cobra.Command{
	Use:   "get-workload [spiffe-id]",
	Short: "Get a workload registration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetWorkload(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var spireWorkloadListCmd = &cobra.Command{
	Use:   "list-workloads",
	Short: "List workload registrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListWorkloads(spireAgentID)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var spireWorkloadAttestCmd = &cobra.Command{
	Use:   "attest-workload [spiffe-id]",
	Short: "Mark a workload as attested",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.AttestWorkload(args[0])
		if err != nil {
			return err
		}
		printSuccess("Workload attested")
		printResult(result)
		return nil
	},
}

var spireWorkloadDeleteCmd = &cobra.Command{
	Use:   "delete-workload [spiffe-id]",
	Short: "Delete a workload registration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		_, err := client.DeleteWorkload(args[0])
		if err != nil {
			return err
		}
		printSuccess("Workload deleted")
		return nil
	},
}

var spireStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get SPIRE integration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.SpireStatus()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	spireBundleCreateCmd.Flags().StringVar(&spireTrustDomain, "trust-domain", "", "Trust domain (required)")
	spireBundleCreateCmd.Flags().StringVar(&spireBundleData, "bundle-data", "", "Bundle data (JWKSet JSON, PEM, or DER)")
	spireBundleCreateCmd.Flags().StringVar(&spireBundleType, "type", "spiffe_x509", "Bundle type")
	spireBundleCreateCmd.Flags().StringVar(&spireSource, "source", "static", "Bundle source (static, web)")
	spireBundleCreateCmd.Flags().StringVar(&spireEndpointURL, "endpoint", "", "Bundle endpoint URL")
	spireBundleCreateCmd.Flags().BoolVar(&spireIsFederated, "federated", false, "Mark as federated trust domain")
	_ = spireBundleCreateCmd.MarkFlagRequired("trust-domain")
	_ = spireBundleCreateCmd.MarkFlagRequired("bundle-data")

	spireBundleUpdateCmd.Flags().StringVar(&spireBundleData, "bundle-data", "", "New bundle data (required)")
	_ = spireBundleUpdateCmd.MarkFlagRequired("bundle-data")

	spireBundleListCmd.Flags().BoolVar(&spireFederatedOnly, "federated", false, "List only federated bundles")

	spireBundleFetchCmd.Flags().StringVar(&spireEndpointURL, "endpoint", "", "Bundle endpoint URL (required)")
	spireBundleFetchCmd.Flags().StringVar(&spireTrustDomain, "trust-domain", "", "Trust domain to save as")
	spireBundleFetchCmd.Flags().BoolVar(&spireIsFederated, "federated", false, "Mark as federated")
	_ = spireBundleFetchCmd.MarkFlagRequired("endpoint")

	spireWorkloadRegisterCmd.Flags().StringVar(&spireSpiffeID, "spiffe-id", "", "SPIFFE ID (required)")
	spireWorkloadRegisterCmd.Flags().StringVar(&spireAgentID, "agent-id", "", "Agent ID (required)")
	spireWorkloadRegisterCmd.Flags().StringVar(&spireTrustDomain, "trust-domain", "", "Trust domain (required)")
	spireWorkloadRegisterCmd.Flags().StringSliceVar(&spireSelectors, "selectors", []string{}, "SPIRE selectors")
	spireWorkloadRegisterCmd.Flags().StringVar(&spireParentID, "parent-id", "", "Parent SPIFFE ID")
	spireWorkloadRegisterCmd.Flags().BoolVar(&spireAutoRegister, "auto-register", true, "Auto-register on attestation")
	_ = spireWorkloadRegisterCmd.MarkFlagRequired("spiffe-id")
	_ = spireWorkloadRegisterCmd.MarkFlagRequired("agent-id")
	_ = spireWorkloadRegisterCmd.MarkFlagRequired("trust-domain")

	spireWorkloadListCmd.Flags().StringVar(&spireAgentID, "agent-id", "", "Filter by agent ID")

	spireCmd.AddCommand(spireBundleCreateCmd)
	spireCmd.AddCommand(spireBundleGetCmd)
	spireCmd.AddCommand(spireBundleListCmd)
	spireCmd.AddCommand(spireBundleUpdateCmd)
	spireCmd.AddCommand(spireBundleVerifyCmd)
	spireCmd.AddCommand(spireBundleDeleteCmd)
	spireCmd.AddCommand(spireBundleFetchCmd)
	spireCmd.AddCommand(spireWorkloadRegisterCmd)
	spireCmd.AddCommand(spireWorkloadGetCmd)
	spireCmd.AddCommand(spireWorkloadListCmd)
	spireCmd.AddCommand(spireWorkloadAttestCmd)
	spireCmd.AddCommand(spireWorkloadDeleteCmd)
	spireCmd.AddCommand(spireStatusCmd)

	rootCmd.AddCommand(spireCmd)
}