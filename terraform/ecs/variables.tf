variable "cluster_ec2_ami_id" {
  description = "the ami for ecs ec2 instances"
  type = string
  default = "ami-00eeedc4036573771"
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
