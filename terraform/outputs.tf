output "s3_bucket_arn" {
  value = aws_s3_bucket.terraform_state.arn
  description = "The ARN of the s3 bucket"
}

output "dynamodb_table_name" {
  value = aws_dynamodb_table.terraform_locks.name
  description = "The name of the DynamoDB table"
}

output "scheduler0_container_registry_arn" {
  value = module.ecr.scheduler0_container_registry_arn
  description = "The arn for the ecr"
}

output "scheduler0_container_registry_id" {
  value = module.ecr.scheduler0_container_registry_id
  description = "ECR registry id"
}

output "scheduler0_container_repository_url" {
  value = module.ecr.scheduler0_container_repository_url
  description = "The repository url"
}

output "scheduler0_container_repository_tags_all" {
  value = module.ecr.scheduler0_container_repository_tags_all
  description = "ECR tags_all"
}