provider "aws" {
  region  = "eu-west-2"
  profile = "scratch"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

resource "aws_instance" "web" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = "r7a.metal-48xl"

  subnet_id              = "subnet-08d487c7b68b31824"
  vpc_security_group_ids = [aws_security_group.web.id]

  tags = {
    Name = "mike-test"
  }
}

resource "aws_security_group" "web" {
  name        = "mike-test"
  description = "Test group - manual"
  vpc_id      = "vpc-0e002ec64373a0ef0"

  tags = {
    Name = "mike-test"
  }
}

resource "aws_launch_template" "web" {
  name          = "mike-test"
  instance_type = "x2gd.medium"
}

resource "aws_launch_configuration" "web" {
  name          = "mike-test"
  image_id      = data.aws_ami.ubuntu.id
  instance_type = "m5dn.xlarge"
}