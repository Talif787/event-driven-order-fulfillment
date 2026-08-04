# One Secrets Manager secret per service, gated. The External Secrets Operator
# (installed in the deploy phase) syncs each into the Kubernetes Secret the Helm
# release reads (order-secrets and so on). Keys are uniform across services so a
# single ExternalSecret template can map them; the Go services ignore the extra
# keys, and only DATABASE_URL changes shape for the Java service (jdbc form).
resource "random_password" "webhook" {
  count = var.enable_data_plane ? 1 : 0

  length  = 32
  special = false
}

resource "aws_secretsmanager_secret" "service" {
  for_each = var.enable_data_plane ? local.services : {}

  name = "${local.name}/${each.key}"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "service" {
  for_each = var.enable_data_plane ? local.services : {}

  secret_id = aws_secretsmanager_secret.service[each.key].id
  secret_string = jsonencode({
    DATABASE_URL = each.value.java ? (
      "jdbc:postgresql://${aws_db_instance.this[each.key].address}:5432/${each.value.db}"
      ) : (
      "postgres://${each.value.username}:${random_password.db[each.key].result}@${aws_db_instance.this[each.key].address}:5432/${each.value.db}?sslmode=require"
    )
    DATABASE_USER          = each.value.username
    DATABASE_PASSWORD      = random_password.db[each.key].result
    PAYMENT_WEBHOOK_SECRET = random_password.webhook[0].result
  })
}
