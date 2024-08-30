# Create a VPC
resource "aws_vpc" "scheduler0_vpc" {
  cidr_block = "10.0.0.0/16"
}

#resource "aws_eip" "scheduler0_vpc_eip_a" {}
#
#resource "aws_eip" "scheduler0_vpc_eip_b" {}

resource "aws_internet_gateway" "scheduler0_vpc_internet_gateway" {
  vpc_id = aws_vpc.scheduler0_vpc.id
}

# Subnet A Setup
resource "aws_subnet" "scheduler0_vpc_public_subnet_a" {
  vpc_id     = aws_vpc.scheduler0_vpc.id
  cidr_block = "10.0.0.0/24"
  availability_zone = var.vpc_public_subnet_availability_zones[0]
  map_public_ip_on_launch = true
}

#resource "aws_subnet" "scheduler0_vpc_private_subnet_a" {
#  vpc_id     = aws_vpc.scheduler0_vpc.id
#  cidr_block = "10.0.16.0/20"
#  availability_zone = var.vpc_public_subnet_availability_zones[0]
#}

resource "aws_route_table" "scheduler0_vpc_public_route_table_a" {
  vpc_id = aws_vpc.scheduler0_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.scheduler0_vpc_internet_gateway.id
  }
}

#resource "aws_route_table" "scheduler0_vpc_private_route_table_a" {
#  vpc_id = aws_vpc.scheduler0_vpc.id
#
#  route {
#    cidr_block = "0.0.0.0/0"
#    nat_gateway_id = aws_nat_gateway.scheduler0_vpc_public_subnet_nat_gateway_a.id
#  }
#}

#resource "aws_nat_gateway" "scheduler0_vpc_public_subnet_nat_gateway_a" {
#  subnet_id = aws_subnet.scheduler0_vpc_public_subnet_a.id
#  connectivity_type = "public"
#  allocation_id = aws_eip.scheduler0_vpc_eni.id
#}

resource "aws_route_table_association" "scheduler0_vpc_public_route_table_subnet_association_a" {
  route_table_id = aws_route_table.scheduler0_vpc_public_route_table_a.id
  subnet_id = aws_subnet.scheduler0_vpc_public_subnet_a.id
}

#resource "aws_route_table_association" "scheduler0_vpc_private_route_table_subnet_association" {
#  route_table_id = aws_route_table.scheduler0_vpc_private_route_table_a.id
#  subnet_id = aws_subnet.scheduler0_vpc_private_subnet_a.id
#}

# Subnet B Setup

#resource "aws_subnet" "scheduler0_vpc_private_subnet_b" {
#  vpc_id     = aws_vpc.scheduler0_vpc.id
#  cidr_block = "10.0.32.0/20"
#  availability_zone = var.vpc_public_subnet_availability_zones[1]
#}

resource "aws_subnet" "scheduler0_vpc_public_subnet_b" {
  vpc_id     = aws_vpc.scheduler0_vpc.id
  cidr_block = "10.0.1.0/24"
  availability_zone = var.vpc_public_subnet_availability_zones[1]
  map_public_ip_on_launch = true
}


resource "aws_route_table" "scheduler0_vpc_public_route_table_b" {
  vpc_id = aws_vpc.scheduler0_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.scheduler0_vpc_internet_gateway.id
  }
}

resource "aws_route_table_association" "scheduler0_vpc_public_route_table_subnet_association_b" {
  route_table_id = aws_route_table.scheduler0_vpc_public_route_table_b.id
  subnet_id = aws_subnet.scheduler0_vpc_public_subnet_b.id
}

#resource "aws_route_table" "scheduler0_vpc_private_route_table" {
#  vpc_id = aws_vpc.scheduler0_vpc.id
#
#  route {
#    cidr_block = "0.0.0.0/0"
#    nat_gateway_id = aws_nat_gateway.scheduler0_vpc_public_subnet_nat_gateway_a.id
#  }
#}
#
#resource "aws_nat_gateway" "scheduler0_vpc_public_subnet_nat_gateway_b" {
#  subnet_id = aws_subnet.scheduler0_vpc_public_subnet_b.id
#  connectivity_type = "public"
#  allocation_id = aws_eip.scheduler0_vpc_eni.id
#}
#
#resource "aws_route_table_association" "scheduler0_vpc_private_route_table_subnet_association" {
#  route_table_id = aws_route_table.scheduler0_vpc_private_route_table.id
#  subnet_id = aws_subnet.scheduler0_vpc_private_subnet_b.id
#}