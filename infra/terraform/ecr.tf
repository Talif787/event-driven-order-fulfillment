# ECR repositories, one per image. Ungated: repositories are effectively free
# (you pay only for stored image data), and CI needs them to push images before
# the cluster exists. Untagged images expire to keep storage small.
resource "aws_ecr_repository" "this" {
  for_each = toset(local.image_names)

  name                 = "${var.project}/${each.value}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = local.tags
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each = aws_ecr_repository.this

  repository = each.value.name
  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images after 14 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 14
        }
        action = { type = "expire" }
      },
    ]
  })
}
