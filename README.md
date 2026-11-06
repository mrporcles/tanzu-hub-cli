# tanzu-hub-cli
Experimental CLI for Tanzu Hub. This is not part of the product and is not covered by global support, nor does it come with any future guarantee of updates.

## Usage
```
  th [command]

Available Commands:
  access-token    Generate an access token
  alert           Manage observability alerts
  completion      Generate the autocompletion script for the specified shell
  foundation      Manage foundation attachment
  help            Help about any command
  license         Managed license keys and license association
  logs            Query and download logs
  region          Manage regions
  vulnerabilities Upload vulnerability or SBOM data to Tanzu Hub

Flags:
  -f, --fqdn string   FQDN of Tanzu Hub [$HUB_FQDN]
  -h, --help          help for th
  -p, --pass string   Password used to authenticate [$HUB_PASSWORD]
  -k, --skip_tls      Skip TLS when connecting to Tanzu Hub [$HUB_SKIP_TLS]
  -u, --user string   Username used to authenticate [$HUB_USERNAME]
  -v, --version       version for th

Use "th [command] --help" for more information about a command.
```

## Installation
Downloads via the [releases page](https://github.com/vmware-tanzu-labs/tanzu-hub-cli/releases).

## Linux/MacOS
Move to a location in $PATH and make executable

```sh
cp th-* $HOME/.local/bin/th
chmod +x th
```

## Roadmap
- Declarative dashboard configuration
