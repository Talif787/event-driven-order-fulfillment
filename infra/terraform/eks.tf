# EKS via the community module. create gates the cluster (control plane and
# node group) behind the data-plane flag. IRSA is enabled so workloads assume
# scoped IAM roles through their service accounts. Access is managed with EKS
# access entries (the v20 default) rather than the aws-auth ConfigMap.
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.24"

  create = var.enable_data_plane

  cluster_name    = local.name
  cluster_version = var.eks_cluster_version

  cluster_endpoint_public_access = true

  vpc_id     = module.network.vpc_id
  subnet_ids = module.network.private_subnets

  enable_irsa = true

  authentication_mode                      = "API_AND_CONFIG_MAP"
  enable_cluster_creator_admin_permissions = true

  cluster_addons = {
    coredns            = {}
    kube-proxy         = {}
    vpc-cni            = {}
    aws-ebs-csi-driver = {}
  }

  eks_managed_node_groups = {
    default = {
      instance_types = var.eks_node_instance_types
      min_size       = var.eks_node_min_size
      max_size       = var.eks_node_max_size
      desired_size   = var.eks_node_desired_size
      capacity_type  = "ON_DEMAND"
    }
  }

  tags = local.tags
}
