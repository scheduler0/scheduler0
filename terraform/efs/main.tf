# Create a new EFS file system
#resource "aws_efs_file_system" "efs" {
#  creation_token = "my-product"
#}

# Create a mount target in each subnet
#resource "aws_efs_mount_target" "alpha" {
#  file_system_id  = aws_efs_file_system.efs.id
#  subnet_id       = "SUBNET_ID"
#  security_groups = [aws_security_group.efs.id]
#}
