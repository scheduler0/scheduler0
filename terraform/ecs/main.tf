# Create a new ECS cluster
resource "aws_ecs_cluster" "scheduler0_ecs_cluster" {
  name = var.cluster_name
}

# Create a new launch configuration with our ECS optimized AMI
resource "aws_launch_configuration" "ecs" {
  name          = var.launch_configuration_name
  image_id      = var.cluster_ec2_ami_id
  instance_type = "t2.micro"

  iam_instance_profile = var.cluster_ec2_instance_type

  user_data = <<-EOF
              #!/bin/bash
              echo ECS_CLUSTER=${aws_ecs_cluster.scheduler0_ecs_cluster.name} >> /etc/ecs/ecs.config
              EOF

  lifecycle {
    create_before_destroy = true
  }
}

# Create an auto scaling group
resource "aws_autoscaling_group" "ecs" {
  desired_capacity     = 2
  launch_configuration = aws_launch_configuration.ecs.id
  max_size             = 2
  min_size             = 0
  vpc_zone_identifier  = [data.aws_subnet_ids.default.id]

  lifecycle {
    create_before_destroy = true
  }
}



