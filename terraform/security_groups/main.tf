# Create a new security group for our ecs instances
resource "aws_security_group" "scheduler0_ecs_cluster_security_group" {
  count = length(data.aws_vpcs.svpcs.ids)

  name        = var.security_group_name
  description = "Allow inbound traffic"
  vpc_id      = data.aws_vpc.svpc[count.index].id

  ingress {
    from_port   = var.cluster_ingress_from_port
    to_port     = var.cluster_ingress_to_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Project: "Scheduler0"
  }
}