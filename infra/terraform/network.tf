# VPC via the community module. create_vpc gates the whole network (and its NAT
# gateways, the only always-on cost here) behind the data-plane flag. Subnets
# carry the tags the AWS Load Balancer Controller and EKS use to discover where
# to place internet-facing and internal load balancers.
module "network" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.8"

  create_vpc = var.enable_data_plane

  name = "${local.name}-vpc"
  cidr = var.vpc_cidr
  azs  = local.azs

  private_subnets = local.private_subnets
  public_subnets  = local.public_subnets

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb"              = "1"
    "kubernetes.io/cluster/${local.name}" = "shared"
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"     = "1"
    "kubernetes.io/cluster/${local.name}" = "shared"
  }

  tags = local.tags
}
