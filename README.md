# inspect-tf-plan

`inspect-tf-plan` is a tool which is designed to be integrated with a CI pipeline to validate that the EC2 instance types
that are being added in via Terraform are valid for the target AWS region, and to stop the changes from being merged into the stable branch. 
By default, Terraform will only error when at the apply
phase for any incorrect instance types, either due to typos or not being available in the target region.

There are linters out there such as [tflint](https://github.com/terraform-linters/tflint) which can check for valid instance types,
but they don't check for regional availability. Regions such as London have a lot less instance types than regions such as Ireland
and this can catch engineers out when raising PR's. See companion script `missing-instances` in this readme for further details.

The tool solves this problem by checking the diff of the Terraform plan and validates that the
instance types are valid for the target AWS region by using the [describe-instance-type-offerings](https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instance-type-offerings.html) API.
It only checks the diffs for the supported resource types (listed below) and for `create` and `update` actions.
The target region defaults to London (eu-west-2) although it can be changed by using the `--region` flag.

The input to this tool is a Terraform plan which has been exported in JSON format:
```shell
terraform plan -out=plan
terraform show -json plan | jq > plan-pretty.json # jq is optional, just to make it more human readable
```

The tool will output each Terraform resource which has an invalid EC2 instance type selected. If any invalid types are detected
the program will exit with a non-zero code. 

Currently supported Terraform resource types:
1. `aws_instance`
2. `aws_launch_template`
3. `aws_launch_configuration`

## How to run locally
Typically, the tool would be run from a CI system after the Terraform plan stage has executed. Example of running locally:
```shell
AWS_PROFILE=profile go run ./cmd/inspect-tf-plan/main.go --plan-path <path-to-plan> [--region <aws-region>] [--log-level <zap-log-level>]

# Example using the defaults of the London region and a log level of ERROR
AWS_PROFILE=profile go run ./cmd/inspect-tf-plan/main.go --plan-path ./plans/unavailable-in-region.json
```


## Useful links
TF plan internals - https://developer.hashicorp.com/terraform/internals/json-format#change-representation

Go helper types for TF plan - https://github.com/hashicorp/terraform-json/blob/main/plan.go

Go SDK - https://pkg.go.dev/github.com/aws/aws-sdk-go-v2


# missing-instances

There is a companion script called `missing-instances` which can be used to list all the EC2 instances which are not available in the
target AWS region. It works by querying all the instance types from the pricing API in the `us-east-1` region and then comparing it with
all the instance types in the target region. For example at the time of writing there were `307` types missing from the London region
whilst only `32` missing from the Ireland region.

## How to run locally
```shell
AWS_PROFILE=profile go run ./cmd/missing-instances/main.go --region <target-region>
```