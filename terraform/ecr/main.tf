resource "aws_ecr_repository" "scheduler0_container_registry" {
  name                 = var.registry_name
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

resource "null_resource" "docker_push" {
  provisioner "local-exec" {
    command = <<EOT
      aws ecr get-login-password --region ${data.aws_region.current.name} | docker login --username AWS --password-stdin ${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.name}.amazonaws.com
      docker build -t my-image -f ../Dockerfile ../
      docker tag my-image:latest ${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.name}.amazonaws.com/${aws_ecr_repository.scheduler0_container_registry.name}:latest
      docker push ${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.name}.amazonaws.com/${aws_ecr_repository.scheduler0_container_registry.name}:latest
    EOT
  }

  depends_on = [aws_ecr_repository.scheduler0_container_registry]
}