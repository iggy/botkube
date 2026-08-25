# Botkube CLI

Command line tool that simplifies working with a Botkube installation in a Kubernetes cluster.

## Installation

```bash
go build -o botkube main.go
```

## Usage

A working kube config is required for all commands.

### Install or upgrade Botkube

```bash
# Install the latest stable version
botkube install

# Install a specific version
botkube install --version 1.14.0

# Install from the Helm chart in this repository (run from the repo root)
botkube install --repo @local
```

The command wraps Helm, so most `helm install`/`helm upgrade` flags are available
(`--set`, `--values`, `--namespace`, `--dry-run`, ...). It streams the agent logs
until the installation settles, unless `--watch=false` is passed.

### Uninstall Botkube

```bash
# Uninstall the default Helm release
botkube uninstall

# Uninstall a specific Helm release
botkube uninstall --release-name botkube-dev
```

### Export the running configuration

```bash
# Print the configuration of the currently installed Botkube
botkube config get

# Print it as JSON
botkube config get -ojson

# Save it to a file
botkube config get > config.yaml
```

## Implementation details

### Configuration export

`config get` cannot read the configuration directly, because it is assembled from
Secrets, ConfigMaps and defaults at agent startup. Instead, the CLI creates a
`botkube-config-exporter` Job in the namespace where Botkube resides, mounting the
same Secrets and ConfigMaps as the Botkube Pod. The Job writes the merged
configuration to a `botkube-config-exporter` ConfigMap, which the CLI reads and then
deletes along with the Job.

The exporter image is derived from the CLI version and can be overridden with the
`--cfg-exporter-image-*` flags.

## Generating documentation

Reference documentation for every command lives in [`docs/`](./docs) and is generated
from the command definitions:

```bash
make gen-docs-cli
```
