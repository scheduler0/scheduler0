provider "aws" {
  region = "us-east-2"
}

#terraform {
#  backend "s3" {
#    bucket = "748201447723-scheduler0-terraform-state"
#    key = "global/s3/terraform.tfstate"
#    region = "us-east-2"
#
#    dynamodb_table = "748201447723-scheduler0-terraform-locks"
#    encrypt = true
#  }
#}

resource "aws_s3_bucket" "terraform_state" {
  bucket = "748201447723-scheduler0-terraform-state"

  force_destroy = true

  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_s3_bucket_versioning" "terraform_state_bucket_versioning" {
  bucket = aws_s3_bucket.terraform_state.id
  versioning_configuration {
    status = "Suspended"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "terraform_state_bucket_server_side_encryption_configuration" {
  bucket = aws_s3_bucket.terraform_state.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_dynamodb_table" "terraform_locks" {
  name = "748201447723-scheduler0-terraform-locks"
  billing_mode = "PAY_PER_REQUEST"
  hash_key = "LockID"
  attribute {
    name = "LockID"
    type = "S"
  }
}


module "iam" {
  source = "./iam"
}

module "vpc" {
  source = "./vpc"
}

module "security_groups" {
  source = "./security_groups"
  vpc_id = module.vpc.vpc_id
}

module "ecr" {
  source = "./ecr"
}

module "ecs" {
  source = "./ecs"
  subnet_id_a = module.vpc.scheduler0_vpc_public_subnet_a
  subnet_id_b = module.vpc.scheduler0_vpc_public_subnet_b
  repository_url = module.ecr.scheduler0_container_repository_url
  instance_profile_name = module.iam.ecs_instance_profile_name
  execution_role_arn = module.iam.ecs_task_execution_role
  security_group_id = module.security_groups.security_group_id
  vpc_id = module.vpc.vpc_id
}