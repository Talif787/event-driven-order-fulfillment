variable "project" {
  description = "Project name, used as a prefix for all resource names."
  type        = string
  default     = "order-fulfillment"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)."
  type        = string
  default     = "dev"
}

variable "region" {
  description = "AWS region."
  type        = string
  default     = "us-east-1"
}

# enable_data_plane is the cost gate. When false (the default) the expensive
# resources (VPC with NAT, EKS, MSK, RDS, and the IAM and secrets that depend on
# them) are not created, so an apply is effectively free and only provisions the
# ECR repositories. A plan with this set to true shows the full stack at no cost;
# only an apply with it true bills. Flip it deliberately to deploy.
variable "enable_data_plane" {
  description = "Create the billable data plane (VPC/NAT, EKS, MSK, RDS)."
  type        = bool
  default     = false
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC."
  type        = string
  default     = "10.0.0.0/16"
}

variable "az_count" {
  description = "Number of availability zones to span."
  type        = number
  default     = 3
}

variable "eks_cluster_version" {
  description = "EKS control plane version."
  type        = string
  default     = "1.30"
}

variable "eks_node_instance_types" {
  description = "Instance types for the managed node group."
  type        = list(string)
  default     = ["t3.large"]
}

variable "eks_node_min_size" {
  description = "Minimum nodes in the managed node group."
  type        = number
  default     = 2
}

variable "eks_node_max_size" {
  description = "Maximum nodes in the managed node group."
  type        = number
  default     = 5
}

variable "eks_node_desired_size" {
  description = "Desired nodes in the managed node group."
  type        = number
  default     = 3
}

variable "kafka_version" {
  description = "MSK Kafka version."
  type        = string
  default     = "3.6.0"
}

variable "kafka_broker_instance_type" {
  description = "MSK broker instance type."
  type        = string
  default     = "kafka.t3.small"
}

variable "kafka_ebs_volume_size" {
  description = "MSK per-broker EBS volume size in GiB."
  type        = number
  default     = 20
}

variable "rds_instance_class" {
  description = "RDS instance class for each service database."
  type        = string
  default     = "db.t4g.micro"
}

variable "rds_engine_version" {
  description = "PostgreSQL engine version for RDS."
  type        = string
  default     = "16.4"
}

variable "rds_allocated_storage" {
  description = "Allocated storage in GiB for each RDS instance."
  type        = number
  default     = 20
}

variable "tags" {
  description = "Additional tags applied to all resources."
  type        = map(string)
  default     = {}
}
