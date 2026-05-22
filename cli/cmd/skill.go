package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
	Long:  "List, create, and manage agent skills and endorsements.",
}

var skillsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all skills",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		category, _ := cmd.Flags().GetString("category")
		result, err := client.ListSkills(category)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var skillsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search skills",
	Long:  "Search skills by name, description, or category.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		query, _ := cmd.Flags().GetString("query")
		category, _ := cmd.Flags().GetString("category")
		result, err := client.SearchSkills(query, category)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var (
	skillName                string
	skillDescription         string
	skillCategory            string
	skillRiskLevel           string
	skillRequiredTrustMin    float64
	skillRequiredProficiency int
)

var skillsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new skill",
	Long: `Create a new skill in the gateway catalog.

Examples:
  eyevesa skills create --name kubernetes --category deployment --risk-level high --required-trust-min 0.7 --required-proficiency 3
  eyevesa skills create --name database --category data --risk-level critical --required-proficiency 2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.CreateSkill(
			skillName,
			skillDescription,
			skillCategory,
			skillRiskLevel,
			skillRequiredTrustMin,
			skillRequiredProficiency,
		)
		if err != nil {
			return err
		}
		printSuccess(fmt.Sprintf("Skill created: %s", skillName))
		printResult(result)
		return nil
	},
}

var skillsGetCmd = &cobra.Command{
	Use:   "get [skill-id]",
	Short: "Get skill details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.GetSkill(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var skillsDeleteCmd = &cobra.Command{
	Use:   "delete [skill-id]",
	Short: "Delete a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.DeleteSkill(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var (
	assignAgentID     string
	assignSkillID     string
	assignProficiency int
)

var skillsAssignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Assign a skill to an agent",
	Long: `Assign a skill to an agent with a proficiency level (1-5).

Examples:
  eyevesa skills assign --agent-id <uuid> --skill-id <uuid> --proficiency 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.AssignSkill(assignAgentID, assignSkillID, assignProficiency)
		if err != nil {
			return err
		}
		printSuccess("Skill assigned to agent")
		printResult(result)
		return nil
	},
}

var (
	endorseAgentID string
	endorseSkillID string
	endorserType   string
	endorserID     string
	endorseComment string
)

var skillsEndorseCmd = &cobra.Command{
	Use:   "endorse",
	Short: "Endorse an agent's skill",
	Long: `Endorse an agent's skill claim. Endorsements from human, agent, or PTV sources.

Examples:
  eyevesa skills endorse --agent-id <uuid> --skill-id <uuid> --endorser-type human --endorser-id admin@company.com --comment "Verified deployment skills"
  eyevesa skills endorse --agent-id <uuid> --skill-id <uuid> --endorser-type ptv --endorser-id hsm-attestation-001`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.EndorseSkill(endorseAgentID, endorseSkillID, endorserType, endorserID, endorseComment)
		if err != nil {
			return err
		}
		printSuccess("Skill endorsed")
		printResult(result)
		return nil
	},
}

var (
	skillVerifyAgentID string
	skillVerifySkillID string
	skillVerifyBy      string
)

var skillsVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify an agent's skill (HITL)",
	Long: `Manually verify an agent's skill claim, typically done by a human approver.

Examples:
  eyevesa skills verify --agent-id <uuid> --skill-id <uuid> --verified-by admin@company.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.VerifySkill(skillVerifyAgentID, skillVerifySkillID, skillVerifyBy)
		if err != nil {
			return err
		}
		printSuccess("Skill verified")
		printResult(result)
		return nil
	},
}

var (
	trustAgentID string
	trustSkillID string
	trustDelta   float64
	trustReason  string
)

var skillsTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "View or adjust per-skill trust scores",
	Long: `View trust scores for an agent's skills, or adjust a skill trust score.

Examples:
  eyevesa skills trust --agent-id <uuid>
  eyevesa skills trust --agent-id <uuid> --skill-id <uuid>
  eyevesa skills trust --agent-id <uuid> --skill-id <uuid> --delta 0.05 --reason "endorsement bonus"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		if trustDelta != 0 && trustAgentID != "" && trustSkillID != "" {
			result, err := client.AdjustSkillTrust(trustAgentID, trustSkillID, trustDelta, trustReason)
			if err != nil {
				return err
			}
			printSuccess("Skill trust adjusted")
			printResult(result)
			return nil
		}
		if trustAgentID != "" {
			result, err := client.GetSkillTrust(trustAgentID, trustSkillID)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		}
		return fmt.Errorf("--agent-id is required")
	},
}

var (
	removeAgentID string
	removeSkillID string
)

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a skill from an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.RemoveSkill(removeAgentID, removeSkillID)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var (
	agentSkillsID string
)

var skillsAgentCmd = &cobra.Command{
	Use:   "agent [agent-id]",
	Short: "List skills assigned to an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.ListAgentSkills(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	skillsListCmd.Flags().String("category", "", "Filter by category")
	skillsSearchCmd.Flags().String("query", "", "Search query")
	skillsSearchCmd.Flags().String("category", "", "Filter by category")
	skillsCreateCmd.Flags().StringVarP(&skillName, "name", "n", "", "Skill name (required)")
	skillsCreateCmd.Flags().StringVarP(&skillDescription, "description", "d", "", "Skill description")
	skillsCreateCmd.Flags().StringVarP(&skillCategory, "category", "C", "general", "Skill category")
	skillsCreateCmd.Flags().StringVar(&skillRiskLevel, "risk-level", "medium", "Risk level: low, medium, high, critical")
	skillsCreateCmd.Flags().Float64Var(&skillRequiredTrustMin, "required-trust-min", 0.5, "Minimum trust score required (0.0-1.0)")
	skillsCreateCmd.Flags().IntVar(&skillRequiredProficiency, "required-proficiency", 1, "Minimum proficiency level required (1-5)")
	_ = skillsCreateCmd.MarkFlagRequired("name")

	skillsAssignCmd.Flags().StringVar(&assignAgentID, "agent-id", "", "Agent ID (required)")
	skillsAssignCmd.Flags().StringVar(&assignSkillID, "skill-id", "", "Skill ID (required)")
	skillsAssignCmd.Flags().IntVar(&assignProficiency, "proficiency", 1, "Proficiency level (1-5)")
	_ = skillsAssignCmd.MarkFlagRequired("agent-id")
	_ = skillsAssignCmd.MarkFlagRequired("skill-id")

	skillsEndorseCmd.Flags().StringVar(&endorseAgentID, "agent-id", "", "Agent ID (required)")
	skillsEndorseCmd.Flags().StringVar(&endorseSkillID, "skill-id", "", "Skill ID (required)")
	skillsEndorseCmd.Flags().StringVar(&endorserType, "endorser-type", "human", "Endorser type: human, agent, ptv")
	skillsEndorseCmd.Flags().StringVar(&endorserID, "endorser-id", "", "Endorser ID (required)")
	skillsEndorseCmd.Flags().StringVar(&endorseComment, "comment", "", "Endorsement comment")
	_ = skillsEndorseCmd.MarkFlagRequired("agent-id")
	_ = skillsEndorseCmd.MarkFlagRequired("skill-id")
	_ = skillsEndorseCmd.MarkFlagRequired("endorser-id")

	skillsVerifyCmd.Flags().StringVar(&skillVerifyAgentID, "agent-id", "", "Agent ID (required)")
	skillsVerifyCmd.Flags().StringVar(&skillVerifySkillID, "skill-id", "", "Skill ID (required)")
	skillsVerifyCmd.Flags().StringVar(&skillVerifyBy, "verified-by", "admin", "Verifier ID")
	_ = skillsVerifyCmd.MarkFlagRequired("agent-id")
	_ = skillsVerifyCmd.MarkFlagRequired("skill-id")

	skillsTrustCmd.Flags().StringVar(&trustAgentID, "agent-id", "", "Agent ID (required)")
	skillsTrustCmd.Flags().StringVar(&trustSkillID, "skill-id", "", "Skill ID (optional, shows all if omitted)")
	skillsTrustCmd.Flags().Float64Var(&trustDelta, "delta", 0, "Trust delta to apply (non-zero adjusts trust)")
	skillsTrustCmd.Flags().StringVar(&trustReason, "reason", "manual adjustment", "Reason for trust adjustment")
	_ = skillsTrustCmd.MarkFlagRequired("agent-id")

	skillsRemoveCmd.Flags().StringVar(&removeAgentID, "agent-id", "", "Agent ID (required)")
	skillsRemoveCmd.Flags().StringVar(&removeSkillID, "skill-id", "", "Skill ID (required)")
	_ = skillsRemoveCmd.MarkFlagRequired("agent-id")
	_ = skillsRemoveCmd.MarkFlagRequired("skill-id")

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsSearchCmd)
	skillsCmd.AddCommand(skillsCreateCmd)
	skillsCmd.AddCommand(skillsGetCmd)
	skillsCmd.AddCommand(skillsDeleteCmd)
	skillsCmd.AddCommand(skillsAssignCmd)
	skillsCmd.AddCommand(skillsEndorseCmd)
	skillsCmd.AddCommand(skillsVerifyCmd)
	skillsCmd.AddCommand(skillsTrustCmd)
	skillsCmd.AddCommand(skillsRemoveCmd)
	skillsCmd.AddCommand(skillsAgentCmd)
	rootCmd.AddCommand(skillsCmd)
}
