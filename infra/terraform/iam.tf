# IRSA: each service's Kubernetes ServiceAccount assumes a scoped IAM role via
# the cluster OIDC provider. App roles grant SASL/IAM access to the MSK cluster.
# Addon roles (Load Balancer Controller, External Secrets) use the community
# module, which carries the well-known policies for those controllers.

data "aws_iam_policy_document" "app_assume" {
  for_each = var.enable_data_plane ? local.services : {}

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:sub"
      values   = ["system:serviceaccount:${local.namespace}:${each.key}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${module.eks.oidc_provider}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "app_msk" {
  count = var.enable_data_plane ? 1 : 0

  statement {
    sid       = "Connect"
    effect    = "Allow"
    actions   = ["kafka-cluster:Connect", "kafka-cluster:DescribeCluster"]
    resources = [aws_msk_cluster.this[0].arn]
  }

  statement {
    sid    = "Topics"
    effect = "Allow"
    actions = [
      "kafka-cluster:CreateTopic",
      "kafka-cluster:DescribeTopic",
      "kafka-cluster:WriteData",
      "kafka-cluster:ReadData",
    ]
    resources = ["${replace(aws_msk_cluster.this[0].arn, ":cluster/", ":topic/")}/*"]
  }

  statement {
    sid       = "Groups"
    effect    = "Allow"
    actions   = ["kafka-cluster:AlterGroup", "kafka-cluster:DescribeGroup"]
    resources = ["${replace(aws_msk_cluster.this[0].arn, ":cluster/", ":group/")}/*"]
  }
}

resource "aws_iam_role" "app" {
  for_each = var.enable_data_plane ? local.services : {}

  name               = "${local.name}-${each.key}-irsa"
  assume_role_policy = data.aws_iam_policy_document.app_assume[each.key].json
  tags               = local.tags
}

resource "aws_iam_role_policy" "app_msk" {
  for_each = var.enable_data_plane ? local.services : {}

  name   = "msk-access"
  role   = aws_iam_role.app[each.key].id
  policy = data.aws_iam_policy_document.app_msk[0].json
}

module "irsa_lb_controller" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  create_role                            = var.enable_data_plane
  role_name                              = "${local.name}-aws-load-balancer-controller"
  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }

  tags = local.tags
}

module "irsa_external_secrets" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  create_role                           = var.enable_data_plane
  role_name                             = "${local.name}-external-secrets"
  attach_external_secrets_policy        = true
  external_secrets_secrets_manager_arns = ["arn:aws:secretsmanager:${var.region}:*:secret:${local.name}/*"]

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["external-secrets:external-secrets"]
    }
  }

  tags = local.tags
}
