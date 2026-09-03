# Amazon CloudWatch Agent onboarding scripts

Scripts to set up the CloudWatch Agent to send OpenTelemetry (OTLP) telemetry to CloudWatch, across AWS, Azure, and GCP platforms.

```
scripts/
  setup.sh          onboarding dispatcher (pick a platform, runs the scripts below)
  aws/setup.sh      AWS-side: IAM trust + install (EC2, ECS, EKS); trust-only for Azure and GCP
  azure/setup.sh    Azure-side: identity + install (VM, AKS)
  gcp/setup.sh      GCP-side: identity + install (GCE VM, GKE)
  install.sh        agent install payload run on a Linux target
  install.ps1       agent install payload run on a Windows target
```

## Platforms

| Platform    | What it does                                                                              |
|-------------|-------------------------------------------------------------------------------------------|
| `aws_ec2`   | Instance-profile role + install via SSM                                                   |
| `aws_ecs`   | Task role + prints the sidecar container definition to add                                |
| `aws_eks`   | Pod Identity (addon + role + association) + the CloudWatch Observability EKS add-on       |
| `azure_vm`  | Managed identity + AWS IAM trust + install via `az vm run-command`                        |
| `azure_aks` | OIDC issuer / workload identity + AWS IAM trust + the CloudWatch Observability Helm chart |
| `gcp_vm`    | Service account discovery + AWS IAM trust + install via `gcloud compute ssh`              |
| `gcp_gke`   | Cluster OIDC issuer + AWS IAM trust + the CloudWatch Observability Helm chart             |

## Quick start

The dispatcher is a convenience wrapper. It fetches the setup scripts over the
network, so it needs `curl` and outbound access.

```sh
# AWS EKS
curl -fsSL https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/main/scripts/setup.sh \
  | CWAGENT_PLATFORM=aws_eks CWAGENT_K8S_CLUSTER_NAME=my-cluster CWAGENT_AWS_REGION=us-east-1 sh

# Azure AKS (from a shell with both the Azure and AWS CLIs)
curl -fsSL https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/main/scripts/setup.sh \
  | CWAGENT_PLATFORM=azure_aks CWAGENT_AZURE_RESOURCE_GROUP=my-rg CWAGENT_K8S_CLUSTER_NAME=my-cluster CWAGENT_AWS_REGION=us-east-1 sh

# GCP GKE (from a shell with both the gcloud and AWS CLIs)
curl -fsSL https://raw.githubusercontent.com/aws/amazon-cloudwatch-agent/main/scripts/setup.sh \
  | CWAGENT_PLATFORM=gcp_gke CWAGENT_GCP_LOCATION=us-east1-b CWAGENT_K8S_CLUSTER_NAME=my-cluster CWAGENT_AWS_REGION=us-east-1 sh
```

Run `./setup.sh` with no platform on a terminal for an interactive wizard.

### AWS platforms

`aws_ec2` / `aws_ecs` / `aws_eks` run entirely in one AWS shell (needs `aws` and
`jq`, with IAM write access). The dispatcher runs `aws/setup.sh`, which does both
trust and install.

### Azure platforms

`azure_vm` / `azure_aks` span two clouds: identity and install need the Azure
CLI (Azure Cloud Shell), trust needs the AWS CLI (AWS CloudShell). In a shell
with both CLIs the dispatcher runs the whole chain. Otherwise it runs whichever
step this shell can and prints a command to paste into the next shell, carrying
the values produced so far.

The flow is `azure/setup.sh` (identity) -> `aws/setup.sh` (trust) -> `azure/setup.sh`
(install). `azure/setup.sh` picks its mode from whether the IAM role ARN is set:

- **ARN set**: identity and install in one run. The agent retries with backoff
  until the AWS trust is in place.
- **ARN unset**: identity only, then stops. Run `aws/setup.sh` to create the role
  and trust, then rerun `azure/setup.sh` with `CWAGENT_AWS_ROLE_ARN` set to install.

### GCP platforms

`gcp_vm` / `gcp_gke` span two clouds the same way: identity and install need the
Google Cloud CLI (`gcloud`), trust needs the AWS CLI. In a shell with both the
dispatcher runs the whole chain, otherwise it runs whichever step this shell can
and prints a command to paste into the next shell.

The flow is `gcp/setup.sh` (identity) -> `aws/setup.sh` (trust) -> `gcp/setup.sh`
(install), and `gcp/setup.sh` picks its mode from `CWAGENT_AWS_ROLE_ARN` exactly
like the Azure script. The identity step is read-only: it discovers the VM's
service account unique ID (`gcp_vm`) or the cluster's OIDC issuer URL
(`gcp_gke`) for the trust step. A VM install is pushed over `gcloud compute
ssh`; a GKE install uses the CloudWatch Observability Helm chart when `helm`,
`kubectl`, and `gke-gcloud-auth-plugin` are present, otherwise the commands are
printed.

## Running the setup scripts directly

The setup scripts document their own inputs and can be run without the
dispatcher.

```sh
# AWS EKS: trust + install in one AWS shell
CWAGENT_PLATFORM=aws_eks CWAGENT_K8S_CLUSTER_NAME=my-cluster CWAGENT_AWS_REGION=us-east-1 \
  ./aws/setup.sh

# Azure AKS with the ARN known up front: identity + install in one Azure shell
CWAGENT_PLATFORM=azure_aks CWAGENT_AWS_ROLE_ARN=arn:aws:iam::123456789012:role/CloudWatchAgentServerRole \
  CWAGENT_AWS_REGION=us-east-1 CWAGENT_AZURE_RESOURCE_GROUP=my-rg CWAGENT_K8S_CLUSTER_NAME=my-cluster \
  ./azure/setup.sh

# GCP GKE with the ARN known up front: identity + install in one GCP shell
CWAGENT_PLATFORM=gcp_gke CWAGENT_AWS_ROLE_ARN=arn:aws:iam::123456789012:role/CloudWatchAgentServerRole \
  CWAGENT_AWS_REGION=us-east-1 CWAGENT_GCP_LOCATION=us-east1-b CWAGENT_K8S_CLUSTER_NAME=my-cluster \
  ./gcp/setup.sh
```

## Environment variables

### Common

| Variable                                | Meaning                                                                  |
|-----------------------------------------|--------------------------------------------------------------------------|
| `CWAGENT_PLATFORM`                      | `aws_ec2` \| `aws_ecs` \| `aws_eks` \| `azure_vm` \| `azure_aks` \| `gcp_vm` \| `gcp_gke` |
| `CWAGENT_AWS_ROLE_NAME`                 | IAM role name (default: `CloudWatchAgentServerRole`)                     |
| `CWAGENT_AWS_REGION`                    | AWS region telemetry is sent to (required)                               |
| `CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH` | When set (`1`/`true`/`yes`/`on`), enable Transaction Search if it is off |

### Per platform

| Variable                           | Applies to              | Meaning                                                                                 |
|------------------------------------|-------------------------|-----------------------------------------------------------------------------------------|
| `CWAGENT_AWS_INSTANCE_ID`          | `aws_ec2`               | EC2 instance ID                                                                         |
| `CWAGENT_AWS_UPDATE_INSTANCE_ROLE` | `aws_ec2`               | When set (`1`/`true`/`yes`/`on`), attach the policy to the role already on the instance |
| `CWAGENT_AWS_ECS_LAUNCH_TYPE`      | `aws_ecs`               | `fargate` \| `ec2` (default: `fargate`)                                                 |
| `CWAGENT_K8S_CLUSTER_NAME`         | `aws_eks`, `azure_aks`, `gcp_gke` | Cluster name                                                                   |
| `CWAGENT_AZURE_RESOURCE_ID`        | `azure_vm`, `azure_aks` | Full ARM resource ID (subscription + group + name in one value); recommended            |
| `CWAGENT_AZURE_SUBSCRIPTION`       | `azure_vm`, `azure_aks` | Subscription ID or name (Cloud Shell's default is used when unset)                      |
| `CWAGENT_AZURE_RESOURCE_GROUP`     | `azure_vm`, `azure_aks` | Resource group (not needed when `CWAGENT_AZURE_RESOURCE_ID` is set)                     |
| `CWAGENT_AZURE_VM_NAME`            | `azure_vm`              | VM name (not needed when `CWAGENT_AZURE_RESOURCE_ID` is set)                            |
| `CWAGENT_AWS_ROLE_ARN`             | `azure_vm`, `azure_aks`, `gcp_vm`, `gcp_gke` | IAM role ARN; when set, the cloud-side setup script also installs  |
| `CWAGENT_AZURE_TENANT_ID`          | `azure_vm`              | Azure tenant ID (produced by `azure/setup.sh`, consumed by `aws/setup.sh`)              |
| `CWAGENT_AZURE_OIDC_ISSUER`        | `azure_aks`             | AKS OIDC issuer URL (produced by `azure/setup.sh`, consumed by `aws/setup.sh`)          |
| `CWAGENT_GCP_PROJECT`              | `gcp_vm`, `gcp_gke`     | Project ID (the gcloud config default is used when unset)                               |
| `CWAGENT_GCP_ZONE`                 | `gcp_vm`                | Zone the VM lives in                                                                    |
| `CWAGENT_GCP_VM_NAME`              | `gcp_vm`                | VM name                                                                                 |
| `CWAGENT_GCP_LOCATION`             | `gcp_gke`               | Zone or region the cluster lives in                                                     |
| `CWAGENT_GCP_SA_UNIQUE_ID`         | `gcp_vm`                | GCP service account unique ID (produced by `gcp/setup.sh`, consumed by `aws/setup.sh`)  |
| `CWAGENT_GCP_OIDC_ISSUER`          | `gcp_gke`               | GKE cluster OIDC issuer URL (produced by `gcp/setup.sh`, consumed by `aws/setup.sh`)    |

The agent runs in the `amazon-cloudwatch` namespace on Kubernetes. Telemetry uses
the default OpenTelemetry configuration (`default:otel`): OTel Container Insights
and logs on the node agent, plus a cluster-scraper for cluster-level metrics.

## Requirements

The quickest way to meet these is a cloud shell, which comes preauthenticated
with the CLIs installed: **AWS CloudShell** for the AWS steps, **Azure Cloud
Shell** for the Azure steps, **Google Cloud Shell** for the GCP steps.

- **AWS platforms:** `aws` (v2.22+ recommended), `jq`, IAM write access. EKS
  installs the CloudWatch Observability EKS add-on (no `helm`/`kubectl` needed).
  Enabling Transaction Search also needs `logs:PutResourcePolicy` and the
  `xray:*TraceSegmentDestination` permissions.
- **Azure platforms:** `az` (v2.47+ recommended). AKS install additionally uses
  `helm` and `kubectl` when present. If either is missing the script prints the
  install command to run from a shell that has them.
- **GCP platforms:** `gcloud`. A GKE install additionally uses `helm`, `kubectl`,
  and `gke-gcloud-auth-plugin` when present; if any is missing the script prints
  the install commands to run from a shell that has them. A GCE VM install is
  pushed over `gcloud compute ssh` (Linux targets), printing the command instead
  when the VM is not reachable over SSH.
- **Targets:** EC2/VM installs fetch `install.sh` (Linux) or `install.ps1`
  (Windows) onto the host. Windows needs `iconv` on the launching shell to build
  the encoded command.

## Notes

- **OTLP traces need Transaction Search**, a per-region account setting. The AWS
  trust step reports when it is off. Set `CWAGENT_AWS_ENABLE_TRANSACTION_SEARCH=1`
  to have it enabled.
- **Safe to re-run.** IAM trust statements and policies are merged, not replaced;
  a role keeps its other trust statements, and an instance keeps its profile.
- **Boolean env vars** accept `1`, `true`, `yes`, or `on` (case-insensitive).
