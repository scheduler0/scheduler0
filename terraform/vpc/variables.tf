variable "vpc_public_subnet_availability_zones" {
  description = "availability zones for vpc subnets"
  type = list(string)
  default   = ["us-east-2a", "us-east-2b", "us-east-2c"]
}
