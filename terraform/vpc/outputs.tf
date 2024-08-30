output "scheduler0_vpc_public_subnet_a_id" {
  description = "Public subnet a id"
  value = aws_subnet.scheduler0_vpc_public_subnet_a.id
}

output "scheduler0_vpc_public_subnet_b_id" {
  description = "Public subnet b id"
  value = aws_subnet.scheduler0_vpc_public_subnet_b.id
}

#output "scheduler0_vpc_public_ip" {
#  description = "Public ip"
#  value = aws_eip.scheduler0_vpc_eni.id
#}

output "scheduler0_vpc_public_subnet_a" {
  description = "Private subnet id a"
  value = aws_subnet.scheduler0_vpc_public_subnet_a.id
}

output "scheduler0_vpc_public_subnet_b" {
  description = "Private subnet id b"
  value = aws_subnet.scheduler0_vpc_public_subnet_b.id
}

output "vpc_id" {
  value = aws_vpc.scheduler0_vpc.id
}