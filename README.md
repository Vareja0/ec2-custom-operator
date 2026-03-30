# AWS EC2 Operator

A Kubernetes operator that lets you manage AWS EC2 instances as native Kubernetes resources. Define an `EC2instance` custom resource and the operator handles provisioning, tracking, and termination on AWS automatically.

## How it works

The operator watches for `EC2instance` objects in your cluster. When one is created, it calls the AWS EC2 API to launch the instance and stores the instance ID and public IP in the resource status. When the object is deleted, the operator terminates the EC2 instance on AWS.

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


Open `dist/chart/values.yaml` and fill in the `secret` section with your credentials:

```yaml
secret:
  enable: true
  awsAccessKeyId: "AKIA..."       # your AWS access key ID
  awsSecretAccessKey: "..."        # your AWS secret access key
  awsRegion: "us-east-1"
```


### 2. Install with Helm

```bash
helm install ec2-operator ./dist/chart \
  --namespace ec2-operator \
  --create-namespace \
  -f dist/chart/values.yaml
```

### 3. Verify the operator is running

```bash
kubectl get pods -n ec2-operator
```

## Creating an EC2 instance

Edit `example.yaml` with your `subnet` and `sshKey`, then apply:

```bash
kubectl apply -f example.yaml
```

Check the status:

```bash
kubectl get ec2instances
```

The `Instance ID` and `Public IP` columns will populate once AWS provisions the instance.

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

The operator will terminate the AWS instance before removing the Kubernetes object.

## Viewing logs

```bash
kubectl logs -n ec2-operator -l control-plane=controller-manager -c manager -f
```

## Uninstalling

```bash
helm uninstall ec2-operator -n ec2-operator
```

> **Note:** CRDs are kept by default after uninstall (`crd.keep: true`). To remove them:
> ```bash
> kubectl delete crd ec2instances.compute.cloud.com
> ```

