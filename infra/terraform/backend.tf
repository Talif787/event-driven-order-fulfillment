# Remote state on S3 with a DynamoDB lock table. Commented out because the
# bucket and table must exist before init can use them (a one-time bootstrap).
# Create them once, then uncomment and run: terraform init -migrate-state.
#
#   aws s3api create-bucket --bucket order-fulfillment-tfstate-<suffix> --region us-east-1
#   aws s3api put-bucket-versioning --bucket order-fulfillment-tfstate-<suffix> \
#     --versioning-configuration Status=Enabled
#   aws dynamodb create-table --table-name order-fulfillment-tflock \
#     --attribute-definitions AttributeName=LockID,AttributeType=S \
#     --key-schema AttributeName=LockID,KeyType=HASH --billing-mode PAY_PER_REQUEST
#
# terraform {
#   backend "s3" {
#     bucket         = "order-fulfillment-tfstate-<suffix>"
#     key            = "dev/terraform.tfstate"
#     region         = "us-east-1"
#     dynamodb_table = "order-fulfillment-tflock"
#     encrypt        = true
#   }
# }
