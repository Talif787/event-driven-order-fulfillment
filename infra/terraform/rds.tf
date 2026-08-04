# One RDS Postgres instance per bounded context (database per service), gated.
# Small single-AZ instances keep the demo affordable; production would raise the
# instance class and enable multi_az. Passwords are generated and never printed;
# the composed connection string lands in Secrets Manager.
resource "aws_db_subnet_group" "this" {
  count = var.enable_data_plane ? 1 : 0

  name       = "${local.name}-db"
  subnet_ids = module.network.private_subnets
  tags       = local.tags
}

resource "aws_security_group" "rds" {
  count = var.enable_data_plane ? 1 : 0

  name_prefix = "${local.name}-rds-"
  description = "Postgres access from within the VPC"
  vpc_id      = module.network.vpc_id

  ingress {
    description = "Postgres"
    from_port   = 5432
    to_port     = 5432
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

resource "random_password" "db" {
  for_each = var.enable_data_plane ? local.services : {}

  length  = 24
  special = false
}

resource "aws_db_instance" "this" {
  for_each = var.enable_data_plane ? local.services : {}

  identifier     = "${local.name}-${each.key}"
  engine         = "postgres"
  engine_version = var.rds_engine_version
  instance_class = var.rds_instance_class

  allocated_storage = var.rds_allocated_storage
  storage_type      = "gp3"
  storage_encrypted = true

  db_name  = each.value.db
  username = each.value.username
  password = random_password.db[each.key].result

  db_subnet_group_name   = aws_db_subnet_group.this[0].name
  vpc_security_group_ids = [aws_security_group.rds[0].id]

  multi_az            = false
  publicly_accessible = false
  skip_final_snapshot = true
  deletion_protection = false
  apply_immediately   = true

  tags = local.tags
}
