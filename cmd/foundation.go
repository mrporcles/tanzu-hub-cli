package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	collectorName  string
	foundationID   string
	foundationName string
	regionName      string
	regionID        string
	attachRegion    string
	legacyAttach   bool
	outputFormat   string
)

type outputEnvelope struct {
	Type string `json:"type" yaml:"type"`
	Spec any    `json:"spec" yaml:"spec"`
}

func printOutput(format string, data any) {
	switch format {
	case "yaml":
		out, err := yaml.Marshal(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal yaml: %s\n", err)
			os.Exit(1)
		}
		fmt.Print(string(out))
	case "json":
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal json: %s\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	}
}

var foundationCmd = &cobra.Command{
	Use: "foundation",
	Short: "Manage foundation attachment",

}

var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Manage foundation attachment",
}

var listFoundationsCmd = &cobra.Command{
	Use:   "list",
	Short: "List management endpoints from Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		endpoints, err := gql.QueryManagementEndpoints(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query management endpoints: %s\n", err)
			os.Exit(1)
		}

		if outputFormat != "text" {
			printOutput(outputFormat, endpoints)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tENVIRONMENT\tVERSION\tLATEST VERSION\tHEALTH\tSTATUS\tFOUNDATION ID\tLAST UPDATED")
		for _, c := range endpoints {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				c.ManagementEndpoint.EndpointName,
				c.Type,
				c.ManagementEndpoint.Environment,
				c.ManagementEndpointCollectorTypeVersion,
				c.LatestAvailableVersion,
				c.HealthStatus,
				c.Status,
				c.ManagementEndpoint.ManagementEndpointID,
				c.LastUpdateTime,
			)
		}
		err = w.Flush()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

type selfManagedCollectorFn func(ctx context.Context, collectorName, collectorType, managementEndpointID string) (map[string]string, error)

func runSelfManagedCollectorCmd(cmd *cobra.Command, selectFn func(*api.GraphQLClient) selfManagedCollectorFn) {
	validateCredentials(cmd)

	client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
		os.Exit(1)
	}

	gql := api.NewGraphQLClient(client, hub_url, skipTls)
	result, err := selectFn(gql)(context.Background(), collectorName, "TAS", foundationID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run collector command: %s\n", err)
		os.Exit(1)
	}

	if outputFormat == "om-yaml" {
		omStr := fmt.Sprintf(`product-name: hub-tas-collector
product-properties:
  .properties.foundation_name:
    value: %s
  .properties.collector_id:
    value: %s
  .properties.csp_org_id:
    value: %s
  .properties.foundation_id:
    value: %s
  .properties.ingestion_url:
    value: %s
  .properties.oauth_client_id:
    value: %s
  .properties.oauth_client_secret:
    value:
      secret: %s
  .properties.frpc_remote_port:
    value: 0
  .properties.skip_ssl_validation:
    value: true
  .properties.hub_ca_certificate:
    value: |
      %s`, result["foundationName"], result["collector-id"], result["org-id"], result["foundation-id"],
			result["ingestion_url"], result["oauth-app-id"], result["oauth-app-secret"],
			strings.ReplaceAll(result["caBundle"], "\n", "\n      "))
		fmt.Println(omStr)
		return
	}

	if outputFormat != "text" {
		printOutput(outputFormat, result)
		return
	}

	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(w, "%s:\t%s\n", k, result[k])
	}
	if err = w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
		os.Exit(1)
	}
}

var attachManualCmd = &cobra.Command{
	Use:   "manual",
	Short: "Attach a self-managed collector to a foundation in Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		if legacyAttach {
			runSelfManagedCollectorCmd(cmd, func(gql *api.GraphQLClient) selfManagedCollectorFn {
				return gql.AttachSelfManagedCollector
			})
			return
		}
		if attachRegion == "" {
			fmt.Fprintln(os.Stderr, "--region is required unless --legacy is set")
			os.Exit(1)
		}
		runSelfManagedCollectorCmd(cmd, func(gql *api.GraphQLClient) selfManagedCollectorFn {
			return func(ctx context.Context, collectorName, _, _ string) (map[string]string, error) {
				return gql.AttachFoundationInRegion(ctx, collectorName, attachRegion)
			}
		})
	},
}

var updateManualCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a self-managed collector on a foundation in Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		runSelfManagedCollectorCmd(cmd, func(gql *api.GraphQLClient) selfManagedCollectorFn {
			return gql.UpdateSelfManagedCollector
		})
	},
}

var deleteFoundationCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a foundation from Tanzu Hub by name",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		fmt.Printf("Are you sure you want to delete foundation %q? [y/N]: ", foundationName)
		var response string
		fmt.Fscan(os.Stdin, &response)
		if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
			fmt.Println("Aborted.")
			return
		}

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		if err := gql.DeleteFoundationByName(context.Background(), foundationName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete foundation: %s\n", err)
			os.Exit(1)
		}

		fmt.Printf("Foundation %q deleted successfully\n", foundationName)
	},
}

var regionCmd = &cobra.Command{
	Use:     "region",
	Short:   "Manage regions",
}

var createRegionCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a region",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		id, err := gql.CreateRegion(context.Background(), regionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create region: %s\n", err)
			os.Exit(1)
		}

		if outputFormat != "text" {
			printOutput(outputFormat, map[string]string{"id": id, "name": regionName})
			return
		}
		fmt.Printf("Region %q created with ID %s\n", regionName, id)
	},
}

var listRegionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List regions and their foundations",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		groups, err := gql.QueryRegions(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query regions: %s\n", err)
			os.Exit(1)
		}

		if outputFormat != "text" {
			printOutput(outputFormat, groups)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tID\tFOUNDATIONS")
		for _, g := range groups {
			names := make([]string, 0, len(g.Foundations))
			for _, f := range g.Foundations {
				names = append(names, f.EntityName)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", g.EntityName, g.EntityID, strings.Join(names, ","))
		}
		if err := w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var deleteRegionCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a region by name or ID",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}
		gql := api.NewGraphQLClient(client, hub_url, skipTls)

		region, err := gql.FindRegion(context.Background(), regionName, regionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to find region: %s\n", err)
			os.Exit(1)
		}
		id, label := region.EntityID, region.EntityName

		if len(region.Foundations) > 0 {
			names := make([]string, 0, len(region.Foundations))
			for _, f := range region.Foundations {
				names = append(names, f.EntityName)
			}
			fmt.Fprintf(os.Stderr, "Cannot delete region %q: it still has %d foundation(s) attached: %s\n",
				label, len(names), strings.Join(names, ", "))
			fmt.Fprintln(os.Stderr, "Delete or move the foundations first.")
			os.Exit(1)
		}

		fmt.Printf("Are you sure you want to delete region %q? [y/N]: ", label)
		var response string
		fmt.Fscan(os.Stdin, &response)
		if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
			fmt.Println("Aborted.")
			return
		}

		if err := gql.DeleteRegion(context.Background(), id); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete region: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("Region %q deleted successfully\n", label)
	},
}

func init() {
	rootCmd.AddCommand(foundationCmd)
	rootCmd.AddCommand(regionCmd)
	regionCmd.AddCommand(createRegionCmd)
	regionCmd.AddCommand(listRegionsCmd)
	regionCmd.AddCommand(deleteRegionCmd)
	deleteRegionCmd.Flags().StringVarP(&regionName, "name", "", "", "Name of the region to delete")
	deleteRegionCmd.Flags().StringVarP(&regionID, "id", "", "", "ID of the region to delete")
	deleteRegionCmd.MarkFlagsOneRequired("name", "id")
	deleteRegionCmd.MarkFlagsMutuallyExclusive("name", "id")
	createRegionCmd.Flags().StringVarP(&regionName, "name", "", "", "Name of the region")
	createRegionCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	_ = createRegionCmd.MarkFlagRequired("name")
	listRegionsCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	foundationCmd.AddCommand(listFoundationsCmd)
	foundationCmd.AddCommand(attachCmd)
	foundationCmd.AddCommand(deleteFoundationCmd)
	attachCmd.AddCommand(attachManualCmd)
	attachCmd.AddCommand(updateManualCmd)
	listFoundationsCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	attachManualCmd.Flags().StringVarP(&collectorName, "name", "", "", "Name for the collector")
	attachManualCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	attachManualCmd.Flags().StringVarP(&attachRegion, "region", "", "", "Name of the region to create the foundation in")
	attachManualCmd.Flags().BoolVarP(&legacyAttach, "legacy", "", false, "Use the legacy attach behavior, which does not use a region")
	_ = attachManualCmd.MarkFlagRequired("name")
	updateManualCmd.Flags().StringVarP(&collectorName, "name", "", "", "Name for the collector")
	updateManualCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format: text, json, yaml")
	_ = updateManualCmd.MarkFlagRequired("name")
	deleteFoundationCmd.Flags().StringVarP(&foundationName, "name", "", "", "Name of the foundation to delete")
	_ = deleteFoundationCmd.MarkFlagRequired("name")
}
