output "region" {
  description = "AWS region."
  value       = var.region
}

output "ecr_repository_urls" {
  description = "ECR repository URLs by image name; use as the Helm global.registry base."
  value       = { for k, r in aws_ecr_repository.this : k => r.repository_url }
}

output "cluster_name" {
  description = "EKS cluster name (empty when the data plane is disabled)."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API server endpoint."
  value       = module.eks.cluster_endpoint
}

output "configure_kubectl" {
  description = "Command to write kubeconfig for the cluster."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${local.name}"
}

output "msk_bootstrap_brokers_sasl_iam" {
  description = "MSK SASL/IAM bootstrap brokers; set as KAFKA_BROKERS in the Helm values."
  value       = try(aws_msk_cluster.this[0].bootstrap_brokers_sasl_iam, null)
}

output "rds_endpoints" {
  description = "RDS endpoint address per service."
  value       = { for k, d in aws_db_instance.this : k => d.address }
}

output "app_irsa_role_arns" {
  description = "IRSA role ARN per service; set as serviceAccount.roleArn in the Helm values."
  value       = { for k, r in aws_iam_role.app : k => r.arn }
}

output "secret_names" {
  description = "Secrets Manager secret name per service, for the ExternalSecret spec."
  value       = { for k, s in aws_secretsmanager_secret.service : k => s.name }
}
