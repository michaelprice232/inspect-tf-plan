package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	tfjson "github.com/hashicorp/terraform-json"
)

const region = "eu-west-2"
const awsProfile = "scratch"

type client struct {
	ec2Client          *ec2.Client
	availableInstances []string
	terraformPlan      tfjson.Plan
}

func newClient() (*client, error) {
	c := client{}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region), config.WithSharedConfigProfile(awsProfile))
	if err != nil {
		return &c, fmt.Errorf("newClient: unable to load SDK config, %w", err)
	}
	c.ec2Client = ec2.NewFromConfig(cfg)

	return &c, nil
}

func (c *client) instanceTypeOfferings() error {
	offeringsOutput, err := c.ec2Client.DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{})
	if err != nil {
		return fmt.Errorf("instanceTypeOfferings: describing instance type offerings: %w", err)
	}

	for _, o := range offeringsOutput.InstanceTypeOfferings {
		c.availableInstances = append(c.availableInstances, string(o.InstanceType))
	}

	log.Printf("%d instance types found in %s", len(c.availableInstances), region)

	return nil
}

func (c *client) parsePlan(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("parsePlan: opening plan file: %w", err)
	}

	err = json.Unmarshal(b, &c.terraformPlan)
	if err != nil {
		return fmt.Errorf("parsePlan: unmarshalling plan file: %w", err)
	}

	return nil
}

func contains(slice []string, search string) bool {
	for _, i := range slice {
		if i == search {
			return true
		}
	}
	return false
}

func (c *client) processResourceChanges() error {
	var foundInvalidInstanceType bool
	supportedChangeTypes := []string{"aws_instance", "aws_launch_template", "aws_launch_configuration"}

	for _, change := range c.terraformPlan.ResourceChanges {

		if contains(supportedChangeTypes, change.Type) && (change.Change.Actions.Update() || change.Change.Actions.Create()) {

			// Only query all available instance types when there is an appropriate TF change that requires it
			if len(c.availableInstances) == 0 {
				err := c.instanceTypeOfferings()
				if err != nil {
					return fmt.Errorf("processResourceChanges: querying for all instance types: %w", err)
				}
			}

			log.Printf("%s %v change found in the plan (%s)", change.Type, change.Change.Actions, change.Address)

			afterPlan := change.Change.After.(map[string]interface{})
			instanceType := afterPlan["instance_type"].(string)

			if !contains(c.availableInstances, instanceType) {
				log.Printf("ERROR: instance type %s for '%s' not valid for this region (%s)", instanceType, change.Address, region)
				foundInvalidInstanceType = true
			}
		}
	}

	if foundInvalidInstanceType {
		return fmt.Errorf("processResourceChanges: found at least invalid instance type. See offenders above")
	}

	return nil
}

func main() {
	planPath := flag.String("plan-path", "", "The path to the Terraform plan file")
	flag.Parse()
	if *planPath == "" {
		log.Fatalf("Usage: %s --plan-file <path-to-tf-plan-file>", os.Args[0])
	}

	c, err := newClient()
	if err != nil {
		log.Fatal(err)
	}

	err = c.parsePlan(*planPath)
	if err != nil {
		log.Fatal(err)
	}

	err = c.processResourceChanges()
	if err != nil {
		log.Fatal(err)
	}
}
