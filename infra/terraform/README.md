# Terraform

Cloud infrastructure for the order-fulfillment platform (Phase 6b): VPC, EKS,
MSK, RDS per service, ECR, and the IAM (IRSA) and Secrets Manager wiring the
Helm charts consume.

## The cost gate

Everything that bills is gated behind one flag, `enable_data_plane`, default
false.

- `terraform apply` with the default creates only the ECR repositories (storage
  billed per image, effectively nothing when empty).
- `terraform plan` with `-var enable_data_plane=true` shows the full stack.
  Plans are free; only an apply bills.
- Flip the flag and apply to deploy for real. Run `terraform destroy` after.

So you can demonstrate the entire plan in an interview at zero cost, and stand
the cluster up only when a specific opportunity warrants it.

## Usage

```bash
cd infra/terraform
terraform init
terraform fmt -check
terraform validate

# free: shows ECR only
terraform plan

# free: shows the whole cluster, MSK, and databases
terraform plan -var enable_data_plane=true

# billable: actually deploys
terraform apply -var enable_data_plane=true
```

Copy `terraform.tfvars.example` to `terraform.tfvars` to set values without
flags. Remote state on S3 is prepared in `backend.tf` (commented, with the
one-time bootstrap commands).

## Wiring outputs into the deploy layer

After an apply with the data plane on, the outputs feed the Helm values:

- `configure_kubectl` writes your kubeconfig.
- `ecr_repository_urls` gives the image locations; set `global.registry`.
- `msk_bootstrap_brokers_sasl_iam` is `KAFKA_BROKERS`.
- `app_irsa_role_arns` populate each service's `serviceAccount.roleArn`.
- `secret_names` feed the ExternalSecret specs so the Operator syncs Secrets
  Manager into the `*-secrets` Kubernetes Secrets the charts read.

## Known follow-up

MSK is configured for SASL/IAM over TLS. The Go and Java services currently
connect to Kafka plaintext with no auth (fine against the compose broker). To
point them at MSK they need client-side SASL/IAM enabled
(aws-msk-iam-sasl-signer for kafka-go, aws-msk-iam-auth for Spring). That is an
application change, tracked with the other deferred hardening.
