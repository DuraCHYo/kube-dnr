# kube-dnr

[Русский](README.ru.md) | **English**

Kubernetes Disaster-and-Recovery - a command-line utility for backing up and restoring Kubernetes cluster resources.

## Features

- **Backup of all cluster resources** with the ability to select specific namespaces
- **Manifest cleanup** of status metadata, system annotations and other fields
- **Saving to a ZIP archive** with an organized structure by namespace
- **Restore into another cluster** with automatic namespace creation
- **Support for all resource types** via the Kubernetes Discovery API
- **Flexible configuration** through a JSON file to exclude resources, namespaces and substrings

## Installation

```bash
go build -o kube-dnr
```

### Docker

Multi-arch images (`linux/amd64`, `linux/arm64`) are published to GitHub Container Registry on every release:

```bash
docker run --rm \
  -v "$PWD/config.json:/config.json:ro" \
  -v "$HOME/.kube/config:/home/nonroot/.kube/config:ro" \
  -v "$PWD/backups:/tmp/backups" \
  ghcr.io/durachyo/kube-dnr:latest \
  --config /config.json backup -A -o /tmp/backups/backup.zip
```

The image is based on distroless, runs as a non-root user and does not contain a shell.

When the container runs inside a Kubernetes pod, no kubeconfig is needed: the CLI automatically uses the pod's ServiceAccount. Grant it the required RBAC permissions.

## Configuration

Configuration is provided via a JSON file specified with the `--config` flag. The configuration file allows you to:

- Exclude specific resource types from the backup
- Exclude specific namespaces
- Exclude resources by substrings in their names
- Configure S3 parameters for uploading backups

Example configuration file `config/config.json`:

```json
{
  "kubernetes": {
    "excludedresources": [
      "Pod",
      "PodMetrics",
      "NodeMetrics",
      "Event",
      "ReplicaSet"
    ],
    "excludednamespaces": [
      "kube-system",
      "kube-public",
      "kube-node-lease"
    ],
    "excludedsubstrings": [
      "sh.helm.release",
      "kube-root-ca.crt"
    ]
  },
  "s3": {
    "bucket": "my-backup-bucket",
    "endpoint": "https://s3.example.com",
    "region": "us-east-1",
    "accesskeyid": "YOUR_ACCESS_KEY",
    "secretaccesskey": "YOUR_SECRET_KEY",
    "usessl": true
  }
}
```

### Configuration fields

#### kubernetes
- `excludedresources` - list of resource types (Kind) to exclude from the backup
- `excludednamespaces` - list of namespaces to skip during backup
- `excludedsubstrings` - list of substrings; resources whose names contain them will be skipped

#### s3
- `bucket` - name of the S3 bucket used to store backups
- `endpoint` - endpoint URL of the S3-compatible storage
- `region` - S3 region
- `accesskeyid` - access key
- `secretaccesskey` - secret key
- `usessl` - use SSL for the connection

## Usage

### Backup mode

Back up all namespaces in the cluster:

```bash
./kube-dnr --config config/config.json backup -A -o backup.zip
```

Back up specific namespaces:

```bash
./kube-dnr --config config/config.json backup -n default,kube-system -o backup.zip
```

Back up using a custom kubeconfig:

```bash
./kube-dnr --config config/config.json backup -A -o backup.zip --kubeconfig /path/to/kubeconfig
```

#### backup command flags

- `-A, --all-namespaces` - Back up all cluster namespaces
- `-n, --namespaces` - Comma-separated list of namespaces to back up
- `-o, --output` - Path to the output archive (required)
- `--kubeconfig` - Path to the kubeconfig file (defaults to the in-cluster ServiceAccount, otherwise `~/.kube/config`)
- `--upload-s3` - Upload the backup to S3 after creation (requires S3 configuration)

#### Global flags

- `--config` - Path to the configuration file (required)

### Restore mode

Restore from an archive into the current cluster (taken from KUBECONFIG):

```bash
./kube-dnr --config config/config.json restore -i backup.zip
```

Restore using a different kubeconfig:

```bash
export KUBECONFIG=/path/to/other-cluster/config
./kube-dnr --config config/config.json restore -i backup.zip
```

Or:

```bash
./kube-dnr --config config/config.json restore -i backup.zip --kubeconfig /path/to/other-cluster/config
```

#### restore command flags

- `-i, --input` - Path to the backup archive (required unless --download-s3 is used)
- `--kubeconfig` - Path to the kubeconfig file (defaults to the in-cluster ServiceAccount, otherwise `~/.kube/config`)
- `--download-s3` - Download the backup from S3 (requires S3 configuration)
- `--s3-object` - Name of the S3 object (the latest object is used if not specified)

#### Global flags

- `--config` - Path to the configuration file (required)

## Archive structure

The backup archive is organized as follows:

```
backup.zip
├── namespaces/
│   ├── default/
│   │   ├── pods/
│   │   │   └── my-pod.yaml
│   │   ├── services/
│   │   │   └── my-service.yaml
│   │   └── deployments/
│   │       └── my-deployment.yaml
│   └── kube-system/
│       └── ...
└── cluster-wide/
    ├── nodes/
    │   └── node1.yaml
    ├── persistentvolumes/
    │   └── pv1.yaml
    └── namespaces/
        └── default.yaml
```

## What is stripped from manifests

During backup, the following fields are removed from resources to ensure portability:

- `status` - resource status
- `metadata.uid` - unique identifier
- `metadata.selfLink` - reference to the resource
- `metadata.creationTimestamp` - creation time
- `metadata.generation` - resource generation
- `metadata.resourceVersion` - resource version
- `metadata.managedFields` - fields managed by controllers
- `metadata.ownerReferences` - fields managed by other applications or operators
- System annotations (kubectl.kubernetes.io/last-applied-configuration and others)
- System labels (pod-template-hash, controller-uid and others)
- Resource-specific fields (e.g. secrets in ServiceAccount)

## Examples

### Full backup and restore

```bash
# Back up all cluster resources
./kube-dnr --config config/config.json backup -A -o full-backup.zip

# Restore into another cluster
export KUBECONFIG=/path/to/new-cluster/config
./kube-dnr --config config/config.json restore -i full-backup.zip
```

### Back up a single application

```bash
# Back up only the application namespace
./kube-dnr --config config/config.json backup -n my-app -o my-app-backup.zip

# Restore into another cluster
./kube-dnr --config config/config.json restore -i my-app-backup.zip --kubeconfig /path/to/new-cluster/config
```

### Backup with upload to S3

```bash
# Back up all resources and upload to S3
./kube-dnr --config config/config.json backup -A -o backup.zip --upload-s3
```

### Restore from S3

```bash
# Restore from the latest backup in S3
./kube-dnr --config config/config.json restore --download-s3

# Restore from a specific object in S3
./kube-dnr --config config/config.json restore --download-s3 --s3-object 20260121-150405/backup.zip
```

## Requirements

- Go 1.27.1 or newer
- Access to a Kubernetes cluster via ServiceAccount (in-cluster) or kubeconfig
- Required RBAC permissions for:
  - Backup: `list` on all resources
  - Restore: `create`, `update`, `get` on all resources

## License

See the LICENSE file
