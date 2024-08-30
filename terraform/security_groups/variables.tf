variable "security_group_name" {
  description = "the name for scheduler0 security groups"
  type = string
  default = "scheduler0_ecs_cluster_security_group"
}

variable "cluster_ingress_from_port" {
  description = "the port for incoming network traffic on container"
  type = number
  default   = 9091
}

variable "cluster_ingress_to_port" {
  description = "the port to forward incoming traffic too"
  type = number
  default   = 9091
}

variable "vpc_id" {
  type = string
  description = "vpc id"
}