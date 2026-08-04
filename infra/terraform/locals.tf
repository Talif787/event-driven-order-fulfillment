data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name = "${var.project}-${var.environment}"

  azs = slice(data.aws_availability_zones.available.names, 0, var.az_count)

  # Non-overlapping /20 private and public subnet CIDRs, one per AZ.
  private_subnets = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 4, i)]
  public_subnets  = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 4, i + 8)]

  # Container images pushed by CI, one ECR repository each. Names match the
  # Helm image references (<service>-<dockerfile-target>).
  image_names = [
    "order-api",
    "order-migrate",
    "order-relay",
    "order-projector",
    "order-orchestrator",
    "inventory-api",
    "inventory-migrate",
    "payment-api",
    "fulfillment-api",
    "fulfillment-migrate",
    "fulfillment-consumer",
    "notification-consumer",
    "notification-migrate",
  ]

  # One database per bounded context. The Kubernetes ServiceAccount name matches
  # the service name, in the order-fulfillment namespace.
  namespace = "order-fulfillment"
  services = {
    order        = { db = "order", username = "order", java = false }
    inventory    = { db = "inventory", username = "inventory", java = false }
    payment      = { db = "payment", username = "payment", java = true }
    fulfillment  = { db = "fulfillment", username = "fulfillment", java = false }
    notification = { db = "notification", username = "notification", java = false }
  }

  tags = merge(
    {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
    },
    var.tags,
  )
}
