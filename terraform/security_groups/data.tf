data "aws_vpc" "default" {
  default = true
}

data "aws_subnet" "default" {
  vpc_id = data.aws_vpc.default.id

  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}