#!/bin/zsh

# Generate plan
terraform plan -out=plan

# Convert plan to JSON format
terraform show -json plan | jq > plan-pretty.json