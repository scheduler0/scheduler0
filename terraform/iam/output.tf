output "ecs_instance_profile_name" {
  value = aws_iam_instance_profile.ecs_instance_profile.name
}

output "ecs_instance_role" {
  value = aws_iam_role.ecs_instance_role.arn
}

output "ecs_task_execution_role" {
  value = aws_iam_role.ecs_task_execution_role.arn
}