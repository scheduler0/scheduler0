data "aws_vpcs" "svpcs" {}

data "aws_vpc" "svpc" {
  count = length(data.aws_vpcs.svpcs.ids)
  id    = tolist(data.aws_vpcs.svpcs.ids)[count.index]
}

data "aws_subnet" "default" {
  count = length(data.aws_vpcs.svpcs.ids)

  vpc_id = data.aws_vpc.svpc[count.index].id

  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.svpc[count.index].id]
  }
}