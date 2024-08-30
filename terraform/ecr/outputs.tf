output "scheduler0_container_registry_arn" {
  value = aws_ecr_repository.scheduler0_container_registry.arn
  description = "The arn for the ecr"
}

output "scheduler0_container_registry_id" {
  value = aws_ecr_repository.scheduler0_container_registry.registry_id
  description = "ECR registry id"
}

output "scheduler0_container_repository_url" {
  value = aws_ecr_repository.scheduler0_container_registry.repository_url
  description = "The repository url"
}

output "scheduler0_container_repository_tags_all" {
  value = aws_ecr_repository.scheduler0_container_registry.tags_all
  description = "ECR tags_all"
}

output "scheduler0_container_repository_name" {
  value = aws_ecr_repository.scheduler0_container_registry.name
  description = "ECR name"
}