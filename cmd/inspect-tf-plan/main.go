package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Adarga-Ltd/lib-golang-common/modules/logging"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	tfjson "github.com/hashicorp/terraform-json"
)

type client struct {
	ec2Client          *ec2.Client
	paginator          *ec2.DescribeInstanceTypeOfferingsPaginator
	availableInstances []string
	terraformPlan      tfjson.Plan
	logger             *logging.Logger
}

func newClient(region, logLevel string) (*client, error) {
	c := client{}

	c.logger = logging.NewLogger(logLevel, os.Stdout)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return &c, fmt.Errorf("newClient: unable to load SDK config, %w", err)
	}
	c.ec2Client = ec2.NewFromConfig(cfg)
	c.paginator = ec2.NewDescribeInstanceTypeOfferingsPaginator(c.ec2Client, &ec2.DescribeInstanceTypeOfferingsInput{})

	return &c, nil
}

func (c *client) instanceTypeOfferings() error {
	for c.paginator.HasMorePages() {
		output, err := c.paginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("instanceTypeOfferings: describing instance type offerings: %w", err)
		}

		for _, o := range output.InstanceTypeOfferings {
			c.availableInstances = append(c.availableInstances, string(o.InstanceType))
		}
	}

	c.logger.Info(fmt.Sprintf("%d instance types found", len(c.availableInstances)))

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

type invalidInstanceType struct {
	address      string
	resourceType string
	instanceType string
}

func (c *client) processResourceChanges() ([]invalidInstanceType, error) {
	offendingInstanceTypes := make([]invalidInstanceType, 0)
	supportedChangeTypes := []string{"aws_instance", "aws_launch_template", "aws_launch_configuration"}

	for _, change := range c.terraformPlan.ResourceChanges {
		if contains(supportedChangeTypes, change.Type) && (change.Change.Actions.Update() || change.Change.Actions.Create()) {

			// Only query all available instance types when there is an appropriate TF change that requires it and
			// if it hasn't been populated in the client already (call a max of once)
			if len(c.availableInstances) == 0 {
				err := c.instanceTypeOfferings()
				if err != nil {
					return nil, fmt.Errorf("processResourceChanges: querying for all instance types: %w", err)
				}
			}

			c.logger.Info(fmt.Sprintf("%s %v change found in the plan (%s)", change.Type, change.Change.Actions, change.Address))

			afterPlan := change.Change.After.(map[string]interface{})
			instanceType := afterPlan["instance_type"].(string)

			// Check if the instance_type in the TF plan is in our retrieved list of available instance types for the AWS region
			if !contains(c.availableInstances, instanceType) {
				c.logger.Error(fmt.Sprintf("ERROR: instance type %s for '%s' not valid for this region", instanceType, change.Address))
				offender := invalidInstanceType{
					address:      change.Address,
					resourceType: change.Type,
					instanceType: instanceType,
				}
				offendingInstanceTypes = append(offendingInstanceTypes, offender)
			}
		}
	}

	return offendingInstanceTypes, nil
}

func main() {
	logLevel := flag.String("log-level", "ERROR", "Zap logger level")
	region := flag.String("region", "eu-west-2", "Which AWS region to target. Defaults to London")
	planPath := flag.String("plan-path", "", "The path to the Terraform plan file")
	flag.Parse()
	if *planPath == "" {
		fmt.Printf("Usage: %s --plan-path <path-to-tf-plan-file>", os.Args[0])
		os.Exit(1)
	}

	c, err := newClient(*region, *logLevel)
	if err != nil {
		c.logger.Fatal(fmt.Sprintf("%v", err))
	}

	err = c.parsePlan(*planPath)
	if err != nil {
		c.logger.Fatal(fmt.Sprintf("%v", err))
	}

	results, err := c.processResourceChanges()
	if err != nil {
		c.logger.Fatal(fmt.Sprintf("%v", err))
	}
	if len(results) != 0 {
		c.logger.Fatal(fmt.Sprintf("At least one invalid instance type has been detected in the plan. See above output for further details"))
	}
}
