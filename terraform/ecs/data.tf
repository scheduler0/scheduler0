data "aws_default_vpc" "default" {}

data "aws_subnet_ids" "default" {
  vpc_id = data.aws_default_vpc.default.id
}
