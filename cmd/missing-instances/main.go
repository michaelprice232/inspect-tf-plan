// Script to check which instance types are not available in the target AWS region
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

type client struct {
	ec2Client        *ec2.Client
	pricingClient    *pricing.Client
	pricingPaginator *pricing.GetAttributeValuesPaginator
	ec2Paginator     *ec2.DescribeInstanceTypeOfferingsPaginator
	allInstanceTypes []string
	regionTypes      []string
	targetRegion     string
}

func newClient(region string) (*client, error) {
	c := client{
		targetRegion: region,
	}

	// Config for 2 different regions as the pricing API is us-east-1 only
	regionConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return &c, fmt.Errorf("newClient: unable to load SDK config for target region, %w", err)
	}
	virginiaConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		return &c, fmt.Errorf("newClient: unable to load SDK config for North Virginia region, %w", err)
	}

	c.ec2Client = ec2.NewFromConfig(regionConfig)
	c.ec2Paginator = ec2.NewDescribeInstanceTypeOfferingsPaginator(c.ec2Client, &ec2.DescribeInstanceTypeOfferingsInput{})

	c.pricingClient = pricing.NewFromConfig(virginiaConfig)
	c.pricingPaginator = pricing.NewGetAttributeValuesPaginator(c.pricingClient, &pricing.GetAttributeValuesInput{
		AttributeName: aws.String("instanceType"),
		ServiceCode:   aws.String("AmazonEC2"),
	})

	return &c, nil
}

func (c *client) everyInstanceType() error {
	for c.pricingPaginator.HasMorePages() {
		results, err := c.pricingPaginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("everyInstanceType: querying attributes from us-east-1 pricing endpoint: %w", err)
		}

		for _, i := range results.AttributeValues {
			// Skip any attributes which contain just the family name (t3) instead of an instance type (t3.micro)
			if !strings.Contains(*i.Value, ".") {
				continue
			}

			c.allInstanceTypes = append(c.allInstanceTypes, *i.Value)
		}
	}

	log.Printf("Total number of instance types returned from pricing API: %d", len(c.allInstanceTypes))
	sort.Strings(c.allInstanceTypes)

	return nil
}

func (c *client) regionInstanceTypes() error {
	for c.ec2Paginator.HasMorePages() {
		output, err := c.ec2Paginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("describing instance type offerings: %w", err)
		}

		for _, o := range output.InstanceTypeOfferings {
			c.regionTypes = append(c.regionTypes, string(o.InstanceType))
		}
	}
	sort.Strings(c.regionTypes)
	log.Printf("Total number of instance types returned from region %s: %d", c.targetRegion, len(c.regionTypes))

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

func main() {
	region := flag.String("region", "eu-west-2", "AWS region to check")
	flag.Parse()
	log.Printf("Using region: %s", *region)

	c, err := newClient(*region)
	if err != nil {
		log.Fatal(err)
	}

	err = c.everyInstanceType()
	if err != nil {
		log.Fatal(err)
	}

	err = c.regionInstanceTypes()
	if err != nil {
		log.Fatal(err)
	}

	// Report any instance types which appear in the pricing API but not available in the target region
	missingTypes := make([]string, 0)
	for _, pricingType := range c.allInstanceTypes {
		if !contains(c.regionTypes, pricingType) {
			missingTypes = append(missingTypes, pricingType)
		}
	}
	log.Printf("%d instances appear in the pricing API but are not available in the %s region", len(missingTypes), c.targetRegion)
	for _, i := range missingTypes {
		log.Printf("%s", i)
	}

}
