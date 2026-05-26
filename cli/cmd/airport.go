package cmd

import (
	"github.com/spf13/cobra"
)

var airportCmd = &cobra.Command{
	Use:   "airport",
	Short: "Where agents meet",
	Long:  "Discover, search, and connect with agents at the airport — the agent meeting place.",
}

var airportSearchCmd = &cobra.Command{
	Use:     "search",
	Short:   "Search for agents at the airport",
	Aliases: []string{"find"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		params := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("capability"); v != "" {
			params["capability"] = v
		}
		if v, _ := cmd.Flags().GetString("skill"); v != "" {
			params["skill"] = v
		}
		if v, _ := cmd.Flags().GetString("status"); v != "" {
			params["status"] = v
		}
		if v, _ := cmd.Flags().GetString("tag"); v != "" {
			params["tag"] = v
		}
		if v, _ := cmd.Flags().GetString("owner"); v != "" {
			params["owner"] = v
		}
		if v, _ := cmd.Flags().GetFloat64("min-trust"); v > 0 {
			params["min_trust"] = v
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			params["limit"] = v
		}
		federated, _ := cmd.Flags().GetBool("federated")
		var result map[string]interface{}
		var err error
		if federated {
			result, err = client.FederatedAirportSearch(params)
		} else {
			result, err = client.AirportSearch(params)
		}
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportOnlineCmd = &cobra.Command{
	Use:     "online",
	Short:   "List agents currently online at the airport",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.AirportListOnline()
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportProfileCmd = &cobra.Command{
	Use:   "profile [agent-id]",
	Short: "Get an agent's airport profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.AirportGetProfile(args[0])
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportHeartbeatCmd = &cobra.Command{
	Use:   "heartbeat [agent-id]",
	Short: "Send a heartbeat for an agent at the airport",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		status, _ := cmd.Flags().GetString("status")
		if status == "" {
			status = "online"
		}
		result, err := client.AirportHeartbeat(args[0], status)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportConnectionsCmd = &cobra.Command{
	Use:   "connections [agent-id]",
	Short: "List an agent's airport connections",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		limit, _ := cmd.Flags().GetInt("limit")
		if limit <= 0 {
			limit = 50
		}
		result, err := client.AirportConnections(args[0], limit)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportUpdateProfileCmd = &cobra.Command{
	Use:   "update-profile [agent-id]",
	Short: "Update an agent's airport profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		update := map[string]interface{}{}
		if v, _ := cmd.Flags().GetString("description"); v != "" {
			update["description"] = v
		}
		if v, _ := cmd.Flags().GetStringArray("tags"); len(v) > 0 {
			update["tags"] = v
		}
		if v, _ := cmd.Flags().GetBool("listed"); cmd.Flags().Changed("listed") {
			update["listed"] = v
		}
		result, err := client.AirportUpdateProfile(args[0], update)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

var airportHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check Airport health and stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		result, err := client.Get("/v1/airport/health")
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	},
}

func init() {
	airportSearchCmd.Flags().String("capability", "", "Filter by capability")
	airportSearchCmd.Flags().String("skill", "", "Filter by skill")
	airportSearchCmd.Flags().String("status", "", "Filter by status (online, offline, busy, idle)")
	airportSearchCmd.Flags().String("tag", "", "Filter by tag")
	airportSearchCmd.Flags().String("owner", "", "Filter by owner")
	airportSearchCmd.Flags().Float64("min-trust", 0, "Minimum trust score")
	airportSearchCmd.Flags().Int("limit", 50, "Max results")
	airportSearchCmd.Flags().Bool("federated", false, "Search trusted federated community nodes instead of only local Airport agents")
	airportHeartbeatCmd.Flags().String("status", "online", "Agent status (online, offline, busy, idle)")
	airportConnectionsCmd.Flags().Int("limit", 50, "Max results")
	airportUpdateProfileCmd.Flags().String("description", "", "Profile description")
	airportUpdateProfileCmd.Flags().StringArray("tags", nil, "Profile tags")
	airportUpdateProfileCmd.Flags().Bool("listed", true, "Whether agent is listed in search")

	airportCmd.AddCommand(airportSearchCmd)
	airportCmd.AddCommand(airportOnlineCmd)
	airportCmd.AddCommand(airportProfileCmd)
	airportCmd.AddCommand(airportHeartbeatCmd)
	airportCmd.AddCommand(airportConnectionsCmd)
	airportCmd.AddCommand(airportUpdateProfileCmd)
	airportCmd.AddCommand(airportHealthCmd)
	addCoreCommand(airportCmd)
}
