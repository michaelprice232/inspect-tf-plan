# README

## Plan internals
https://developer.hashicorp.com/terraform/internals/json-format#change-representation

## Helper types for plan
https://github.com/hashicorp/terraform-json/blob/main/plan.go

## Retries / backoff are enabled in the Go SDK (3 retries, max 20s delay):
https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/retries-timeouts/


## Linters are available but these only check a static instance list and not region availability:
https://github.com/terraform-linters/tflint
https://github.com/terraform-linters/tflint-ruleset-aws/blob/master/rules/models/aws_instance_invalid_type.go#L13

## Go EC2 SDK
https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2
