# MSK (managed Kafka), gated. Brokers sit in the private subnets, one per AZ.
# In-transit encryption is TLS and client auth is SASL/IAM, so workloads
# authenticate with their IRSA role rather than a static secret. NOTE: the Go
# and Java services currently connect plaintext with no auth; pointing them at
# MSK requires enabling SASL/IAM on the clients (aws-msk-iam-sasl-signer for
# kafka-go, aws-msk-iam-auth for Spring). That app-side change is a follow-up.
resource "aws_security_group" "msk" {
  count = var.enable_data_plane ? 1 : 0

  name_prefix = "${local.name}-msk-"
  description = "MSK broker access from within the VPC"
  vpc_id      = module.network.vpc_id

  ingress {
    description = "Kafka SASL/IAM over TLS"
    from_port   = 9098
    to_port     = 9098
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.tags
}

resource "aws_msk_configuration" "this" {
  count = var.enable_data_plane ? 1 : 0

  name           = "${local.name}-config"
  kafka_versions = [var.kafka_version]

  server_properties = <<-PROPS
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    num.partitions=3
  PROPS
}

resource "aws_msk_cluster" "this" {
  count = var.enable_data_plane ? 1 : 0

  cluster_name           = local.name
  kafka_version          = var.kafka_version
  number_of_broker_nodes = var.az_count

  broker_node_group_info {
    instance_type   = var.kafka_broker_instance_type
    client_subnets  = module.network.private_subnets
    security_groups = [aws_security_group.msk[0].id]

    storage_info {
      ebs_storage_info {
        volume_size = var.kafka_ebs_volume_size
      }
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.this[0].arn
    revision = aws_msk_configuration.this[0].latest_revision
  }

  client_authentication {
    sasl {
      iam = true
    }
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  tags = local.tags
}
