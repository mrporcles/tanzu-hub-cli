package cmd

import (
	"github.com/spf13/cobra"
)
// version is set at build time via -ldflags "-X .../cmd.version=...".
var version = "dev"

var(
	username string
	password string
	hub_url string
	skipTls bool
	debug bool
)


var rootCmd = &cobra.Command{
	Use:   "th",
	Version: version,
	Short: "Tanzu Hub CLI",
	Long:  `Unofficial CLI for Tanzu Hub `,
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&username, "user", "u", "", "Username used to authenticate [$HUB_USERNAME]")
	rootCmd.PersistentFlags().StringVarP(&password, "pass", "p", "", "Password used to authenticate [$HUB_PASSWORD]")
	rootCmd.PersistentFlags().StringVarP(&hub_url, "fqdn", "f", "", "FQDN of Tanzu Hub [$HUB_TARGET]")
	rootCmd.PersistentFlags().BoolVarP(&skipTls, "skip_tls", "k", false, "Skip TLS when connecting to Tanzu Hub [$HUB_SKIP_TLS]")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging of HTTP requests and responses")
}