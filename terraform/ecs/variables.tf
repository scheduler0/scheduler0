variable "cluster_ec2_ami_id" {
  description = "the ami for ecs ec2 instances"
  type = string
  default = "ami-091d1357d7f80b1fa"
}

variable "cluster_ec2_instance_type" {
  description = "the ec2 instance type"
  type = string
  default = "t2-micro"
}

variable "cluster_name" {
  description = "the cluster name"
  type = string
  default = "scheduler0_ecs_cluster"
}

variable "launch_configuration_name" {
  description = "the cluster lunch configuration name"
  type        = string
  default     = "scheduler0_launch_configuration"
}

variable "subnet_id_a" {
  type = string
  description = "subnet id"
}

variable "subnet_id_b" {
  type = string
  description = "subnet id"
}

variable "image_tag" {
  default = "latest"
}

variable "repository_url" {
  type = string
  description = "The url of container repository"
}


variable "instance_profile_name" {
  type = string
  description = "The instance profile name"
}


variable "execution_role_arn" {
  type = string
  description = "The instance execution role arn"
}

variable "security_group_id" {
  type = string
}

variable "vpc_id" {
  type = string
}