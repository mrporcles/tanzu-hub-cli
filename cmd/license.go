package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/laidbackware/tanzu-hub-cli/api"
	"github.com/spf13/cobra"
)

var (
	licenseKey     string
	licenseID      string
	foundationIDs  string
)

var licensesCmd = &cobra.Command{
	Use:   "license",
	Short: "Managed license keys and license association",
}

var listLicensesCmd = &cobra.Command{
	Use:   "list",
	Short: "List licenses from Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		licenses, err := gql.QueryLicenses(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query licenses: %s\n", err)
			os.Exit(1)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tPRODUCT ID\tPRODUCT DESCRIPTION\tVERSION\tFOUNDATIONS\tEXPIRATION")
		for _, l := range licenses {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				l.ID,
				l.ProductID,
				l.ProductDescription,
				l.LicenseVersion,
				l.FoundationCount,
				l.Expiration,
			)
		}
		err = w.Flush()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var addLicenseCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a license to Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		result, err := gql.AddLicense(context.Background(), licenseKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add license: %s\n", err)
			os.Exit(1)
		}

		if !result.Success {
			fmt.Fprintf(os.Stderr, "License verification failed: %s\n", result.VerificationFailure)
			os.Exit(1)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tPRODUCT ID\tPRODUCT DESCRIPTION\tVERSION\tEXPIRATION")
		l := result.License
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			l.ID, l.ProductID, l.ProductDescription, l.LicenseVersion, l.Expiration,
		)
		err = w.Flush()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

var deleteLicenseCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a license from Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		ok, err := gql.DeleteLicense(context.Background(), licenseID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete license: %s\n", err)
			os.Exit(1)
		}

		if !ok {
			fmt.Fprintln(os.Stderr, "Delete reported success=false")
			os.Exit(1)
		}

		fmt.Printf("License %s deleted successfully\n", licenseID)
	},
}

var applyLicenseCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a license to one or more foundations in Tanzu Hub",
	Run: func(cmd *cobra.Command, args []string) {
		validateCredentials(cmd)

		client, err := api.GetAccessToken(username, password, hub_url, skipTls, debug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate: %s\n", err)
			os.Exit(1)
		}

		ids := strings.Split(foundationIDs, ",")
		for i, id := range ids {
			ids[i] = strings.TrimSpace(id)
		}

		gql := api.NewGraphQLClient(client, hub_url, skipTls)
		ids, err = gql.ResolveFoundationEntityIDs(context.Background(), ids)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve foundations: %s\n", err)
			os.Exit(1)
		}
		result, err := gql.ApplyLicenseToFoundations(context.Background(), licenseID, ids)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to apply license: %s\n", err)
			os.Exit(1)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "FOUNDATION ID\tSTATUS\tWORKFLOW ID\tERRORS")
		for _, r := range result.LicenseApplicationResults {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				r.FoundationID,
				r.Status,
				r.WorkflowID,
				strings.Join(r.Errors, "; "),
			)
		}
		err = w.Flush()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush buffer: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(licensesCmd)
	licensesCmd.AddCommand(listLicensesCmd)
	licensesCmd.AddCommand(addLicenseCmd)
	licensesCmd.AddCommand(deleteLicenseCmd)
	licensesCmd.AddCommand(applyLicenseCmd)
	addLicenseCmd.Flags().StringVarP(&licenseKey, "key", "", "", "License key to add")
	_ = addLicenseCmd.MarkFlagRequired("key")
	deleteLicenseCmd.Flags().StringVarP(&licenseID, "license-id", "", "", "ID of the license to delete")
	_ = deleteLicenseCmd.MarkFlagRequired("license-id")
	applyLicenseCmd.Flags().StringVarP(&licenseID, "license-id", "", "", "ID of the license to apply")
	applyLicenseCmd.Flags().StringVarP(&foundationIDs, "foundation-ids", "", "", "Comma-separated list of foundation IDs to apply the license to")
	_ = applyLicenseCmd.MarkFlagRequired("license-id")
	_ = applyLicenseCmd.MarkFlagRequired("foundation-ids")
}
