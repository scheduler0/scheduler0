# Create a new security group for our ecs instances
resource "aws_security_group" "scheduler0_ecs_cluster_security_group" {
  name        = var.security_group_name
  description = "Allow inbound traffic"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = var.cluster_ingress_from_port
    to_port     = var.cluster_ingress_to_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = "22"
    to_port     = "22"
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = "443"
    to_port     = "443"
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