package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	tfjson "github.com/hashicorp/terraform-json"
)

const region = "eu-west-2"
const awsProfile = "scratch"

// const filePath = "./plans/aws-instance-update.json"
const filePath = "./plans/aws-instance-create.json"

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region), config.WithSharedConfigProfile(awsProfile))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	svc := ec2.NewFromConfig(cfg)

	// todo: only call this if an applicable resource type is found
	offeringsOutput, err := svc.DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{})
	if err != nil {
		log.Fatalf("describing instance type offerings: %v", err)
	}
	offerings := make([]string, 0)
	for _, o := range offeringsOutput.InstanceTypeOfferings {
		offerings = append(offerings, string(o.InstanceType))
	}
	log.Printf("%d instance types found in %s", len(offerings), region)

	plan := tfjson.Plan{}

	b, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("opening plan file: %v", err)
	}

	err = json.Unmarshal(b, &plan)
	if err != nil {
		log.Fatalf("unmarshalling plan file: %v", err)
	}

	for _, c := range plan.ResourceChanges {
		if c.Type == "aws_instance" && (c.Change.Actions.Update() || c.Change.Actions.Create()) {
			log.Printf("aws_instance %v change found in the plan (%s)", c.Change.Actions, c.Address)

			instance := c.Change.After.(map[string]interface{})
			instanceType := instance["instance_type"].(string)

			if !isValidInstance(offerings, instanceType) {
				log.Fatalf("instance type %s not valid for this region (%s)", instanceType, region)
			}
		}
	}
}

// isValidInstance returns whether allInstances contains instance
func isValidInstance(allInstances []string, instance string) bool {
	for _, i := range allInstances {
		if i == instance {
			return true
		}
	}
	return false
}
