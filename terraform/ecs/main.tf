resource "aws_ecs_cluster" "scheduler0_ecs_cluster" {
  name = var.cluster_name

  depends_on = [
    aws_lb.scheduler0_lb,
    aws_lb_target_group.scheduler0_target_group,
    aws_lb_listener.scheduler0_listener
  ]
}


resource "aws_launch_configuration" "ecs" {
  name          = var.launch_configuration_name
  image_id      = var.cluster_ec2_ami_id
  instance_type = "t2.micro"

  iam_instance_profile = var.instance_profile_name

  user_data = <<-EOF
            #!/bin/bash
            echo ECS_CLUSTER=${aws_ecs_cluster.scheduler0_ecs_cluster.name} >> /etc/ecs/ecs.config;
            EOF

  security_groups = [var.security_group_id]


  lifecycle {
    create_before_destroy = true
  }
}

locals {
  image_uri = "${var.repository_url}:${var.image_tag}"
  subnets = [var.subnet_id_a, var.subnet_id_b]
}

resource "aws_autoscaling_group" "ecs" {
  count = 1

  desired_capacity     = 2
  launch_configuration = aws_launch_configuration.ecs.id
  max_size             = 2
  min_size             = 0
  vpc_zone_identifier  = local.subnets

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_cloudwatch_log_group" "ecs_log_group" {
  name              = "/ecs/scheduler0-ecs-service"
  retention_in_days = 30
}

resource "aws_ecs_task_definition" "scheduler0" {
  container_definitions = jsonencode([{
      name      = "scheduler0"
      image     = local.image_uri
      cpu       = 10
      memory    = 512
      essential = true
      portMappings = [
        {
          containerPort = 9091
          hostPort      = 9091
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = "/ecs/scheduler0-ecs-service"
          awslogs-region        = "us-east-2"
          awslogs-stream-prefix = "ecs"
        }
      }
      environment = [
        {
          name  = "SCHEDULER0_SECRET_KEY"
          value = "AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94"
        },
        {
          name  = "SCHEDULER0_AUTH_PASSWORD"
          value = "admin"
        },
        {
          name  = "SCHEDULER0_AUTH_USERNAME"
          value = "admin"
        },
        {
          name  = "SCHEDULER0_PORT"
          value = "9091"
        },
        {
          name  = "SCHEDULER0_BOOTSTRAP"
          value = "true"
        },
        {
          name  = "SCHEDULER0_NODE_ID"
          value = "1"
        },
        {
          name  = "SCHEDULER0_RAFT_ADDRESS"
          value = "127.0.0.1:7071"
        },
        {
          name  = "SCHEDULER0_REPLICAS"
          value = "[{\"nodeId\": 1, \"raft_address\": \"127.0.0.1:7071\", \"address\": \"http://127.0.0.1:9091\"}]"
        }
      ]
    }])

  family          = "scheduler0"
  cpu             = "256"
  memory          = "512"
  network_mode    = "host"
  requires_compatibilities = ["EC2"]
  execution_role_arn = var.execution_role_arn
}

resource "aws_lb" "scheduler0_lb" {
  name               = "scheduler0"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [var.security_group_id]
  subnets            = [var.subnet_id_a, var.subnet_id_b]
}

resource "aws_lb_target_group" "scheduler0_target_group" {
  name     = "scheduler0-target-group"
  port     = 9091
  protocol = "HTTP"
  vpc_id   = var.vpc_id
}

resource "aws_lb_listener" "scheduler0_listener" {
  load_balancer_arn = aws_lb.scheduler0_lb.arn
  port              = "9091"
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.scheduler0_target_group.arn
  }
}

resource "aws_ecs_service" "scheduler0" {
  name = "scheduler0-service"
  cluster = aws_ecs_cluster.scheduler0_ecs_cluster.id
  task_definition = aws_ecs_task_definition.scheduler0.arn
  desired_count   = 1
  launch_type     = "EC2"

  load_balancer {
    target_group_arn = aws_lb_target_group.scheduler0_target_group.arn
    container_name   = "scheduler0"
    container_port   = 9091
  }
}



