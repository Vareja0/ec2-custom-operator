# AWS EC2 Operator

A Kubernetes operator that lets you manage AWS EC2 instances as native Kubernetes resources. Define an `EC2instance` custom resource and the operator handles provisioning, tracking, and termination on AWS automatically.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                        │
│                                                                  │
│  ┌─────────────────┐        ┌──────────────────────────────┐    │
│  │  User / kubectl │        │      namespace: monitoring    │    │
│  └────────┬────────┘        │                              │    │
│           │ apply/delete     │  ┌──────────┐  ┌─────────┐  │    │
│           ▼                  │  │Prometheus│  │ Grafana │  │    │
│  ┌─────────────────────┐     │  └────┬─────┘  └────┬────┘  │    │
│  │ namespace: ec2-op.. │     │       │  scrape       │ query │    │
│  │                     │     └───────┼───────────────┼───────┘    │
│  │  ┌───────────────┐  │             │               │            │
│  │  │  EC2instance  │  │◀────────────┘               │            │
│  │  │      CR       │  │  ServiceMonitor              │            │
│  │  └──────┬────────┘  │  :8443/metrics               │            │
│  │         │ watch      │                              │            │
│  │         ▼            │  ┌───────────────────────┐  │            │
│  │  ┌────────────────┐  │  │  cert-manager (TLS)   │  │            │
│  │  │   Controller   │◀─┼──│  metrics-server-cert  │  │            │
│  │  │   Manager      │  │  └───────────────────────┘  │            │
│  │  └──────┬─────────┘  │                              │            │
│  └─────────┼────────────┘                              │            │
└────────────┼──────────────────────────────────────────┼────────────┘
             │ AWS SDK                                   │
             ▼                                           │
  ┌──────────────────────┐                    Dashboards │
  │      AWS EC2 API     │◀───────────────────────────── ┘
  │  RunInstances        │
  │  DescribeInstances   │
  │  TerminateInstances  │
  └──────────────────────┘
```

## Workflow

### Instance lifecycle

```
kubectl apply -f instance.yaml
        │
        ▼
  Operator adds finalizer
        │
        ▼
  createEc2Instance() ──► AWS RunInstances API
        │
        ▼
  Wait for "running" state
        │
        ▼
  Status updated: InstanceID, PublicIP, PrivateIP
  Metric: ec2_instances_created_total++
  Metric: ec2_instance_running{id, name} = 1
        │
        ▼
  Spec change detected? ──► Terminate + Recreate
        │                    Metric: ec2_instances_replaced_total++
        ▼
kubectl delete ec2instance <name>
        │
        ▼
  DeletionTimestamp set → finalizer triggers
        │
        ▼
  deleteEc2Instance() ──► AWS TerminateInstances API
        │
        ▼
  Finalizer removed → object garbage collected
  Metric: ec2_instances_deleted_total++
  Metric: ec2_instance_running{id, name} deleted
```

## Prerequisites

- Kubernetes cluster v1.11.3+
- `kubectl` configured against the cluster
- Helm v3+
- AWS credentials with EC2 permissions (`ec2:RunInstances`, `ec2:DescribeInstances`, `ec2:TerminateInstances`)

## Installation

### 1. Clone the repository

```bash
git clone https://github.com/vareja0/operator-repo.git
cd operator-repo
```

### 2. Fill in your AWS credentials

Open `dist/chart/values.yaml` and fill in the `secret` section:

```yaml
secret:
  enable: true
  awsAccessKeyId: "AKIA..."
  awsSecretAccessKey: "..."
  awsRegion: "us-east-1"
```

### 3. Deploy everything

```bash
./deploy.sh create
```

This installs in order:
1. Prometheus Operator CRDs
2. cert-manager (for metrics TLS)
3. The operator Helm chart (namespace: `ec2-operator`)
4. Grafana (namespace: `monitoring`)

To tear everything down:

```bash
./deploy.sh delete
```

### 4. Verify the operator is running

```bash
kubectl get pods -n ec2-operator
```

## Observability

Prometheus and Grafana are deployed in the `monitoring` namespace.

```bash
# Access Grafana
kubectl port-forward svc/grafana 3000:80 -n monitoring

# Access Prometheus
kubectl port-forward svc/prometheus-operated 9090:9090 -n monitoring
```

Default Grafana credentials: `admin` / `admin`

Import the dashboards from the `grafana/` directory:
- `grafana/controller-runtime-metrics.json` — reconciliation & work queue metrics
- `grafana/controller-resources-metrics.json` — CPU & memory usage
- `grafana/custom-metrics/custom-metrics-dashboard.json` — EC2 instance metrics

### Custom metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ec2_instances_created_total` | Counter | Total EC2 instances successfully created |
| `ec2_instances_deleted_total` | Counter | Total EC2 instances successfully deleted |
| `ec2_instances_replaced_total` | Counter | Total EC2 instances replaced due to spec change |
| `ec2_operation_errors_total` | Counter | Errors per operation (`create`, `delete`, `describe`) |
| `ec2_instance_running` | Gauge | Instance running state — `1` = running, `0` = not running |

## Creating an EC2 instance

Edit `example.yaml` with your `subnet` and `sshKey`, then apply:

```bash
kubectl apply -f example.yaml
```

Check the status:

```bash
kubectl get ec2instances
```

The `Instance ID` and `Public IP` columns populate once AWS provisions the instance.

## EC2instance spec reference

| Field | Required | Description |
|---|---|---|
| `instanceName` | yes | Name tag assigned to the EC2 instance |
| `type` | yes | EC2 instance type (e.g. `t3.micro`) |
| `region` | yes | AWS region (e.g. `us-east-1`) |
| `sshKey` | yes | Name of the EC2 key pair for SSH access |
| `subnet` | yes | Subnet ID where the instance will be launched |
| `storage.rootVolume.size` | yes | Root volume size in GiB |
| `amiID` | no | AMI ID — must match the instance type architecture |
| `avaibilityZone` | no | Availability zone (e.g. `us-east-1a`) |
| `associatePublicIp` | no | Whether to assign a public IP (default: `false`) |
| `securityGroups` | no | List of security group IDs |
| `storage.rootVolume.type` | no | Volume type (e.g. `gp3`) |
| `storage.rootVolume.encrypted` | no | Encrypt the root volume |
| `storage.additionalVolumes` | no | List of extra EBS volumes to attach |
| `tags` | no | Map of AWS tags to apply to the instance |
| `userData` | no | User data script to run on launch |

## Deleting an EC2 instance

```bash
kubectl delete ec2instance minimal-instance
```

The operator terminates the AWS instance before removing the Kubernetes object.

## Viewing logs

```bash
kubectl logs -n ec2-operator -l control-plane=controller-manager -c manager -f
```

## Uninstalling

```bash
./deploy.sh delete
```

> **Note:** CRDs are kept by default after uninstall (`crd.keep: true`). To remove them manually:
> ```bash
> kubectl delete crd ec2instances.compute.cloud.com
> ```

## Changelog

### Recent changes

- **Observability**: Prometheus + Grafana deployed in dedicated `monitoring` namespace with cross-namespace ServiceMonitor discovery
- **Security**: cert-manager integration for TLS on the metrics endpoint (`certmanager.enable: true`)
- **Deploy script**: `deploy.sh create|delete` automates full stack deployment and teardown
- **Metrics scraping**: Prometheus scrape interval set to `15s`; Grafana datasource interval aligned to `15s`
- **RBAC**: Fixed metrics reader binding to grant Prometheus service account in `monitoring` namespace access to operator metrics
- **Helm**: Guarded Prometheus resources with CRD capabilities check; `--force-conflicts` added to upgrade command
- **Docker**: Buildx integration for multi-platform image builds
