package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Adarga-Ltd/lib-golang-common/modules/logging"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
)

type mockDescribeInstanceTypeOfferingsPaginator struct {
	PageNum int
	Pages   []*ec2.DescribeInstanceTypeOfferingsOutput
}

func (m *mockDescribeInstanceTypeOfferingsPaginator) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *mockDescribeInstanceTypeOfferingsPaginator) NextPage(_ context.Context, _ ...func(options *ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	var output *ec2.DescribeInstanceTypeOfferingsOutput
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++

	return output, nil
}

func Test_instanceTypeOfferings(t *testing.T) {
	pager := &mockDescribeInstanceTypeOfferingsPaginator{
		Pages: []*ec2.DescribeInstanceTypeOfferingsOutput{
			{InstanceTypeOfferings: []types.InstanceTypeOffering{
				{InstanceType: "t3.micro"},
				{InstanceType: "m5.large"},
				{InstanceType: "c7.medium"},
			}}},
	}

	c := client{paginator: pager, logger: logging.NewLogger("ERROR", os.Stdout)}

	err := c.instanceTypeOfferings()
	assert.NoError(t, err, "expected no error when querying instance type offerings")
	assert.Equal(t, 3, len(c.availableInstances), "expected to receive 3 instance types")
}

func Test_newClient(t *testing.T) {
	c, err := newClient("eu-west-2", "INFO")
	assert.NoError(t, err, "expected no error when creating new client")
	assert.NotNilf(t, c.logger, "expected logger to not be nil")
	assert.NotNilf(t, c.ec2Client, "expected ec2 client to not be nil")
	assert.NotNilf(t, c.paginator, "expected ec2 paginator to not be nil")
	assert.Equal(t, 0, len(c.availableInstances), "expected available instances to be initially empty")
}

func Test_parsePlan(t *testing.T) {
	cases := []struct {
		name                    string
		path                    string
		expectedResourceChanges int
		expectedError           bool
	}{
		{name: "happy-path", path: "./testdata/aws-instance-create.json", expectedResourceChanges: 2, expectedError: false},
		{name: "malformed-plan", path: "./testdata/malformed.json", expectedResourceChanges: 0, expectedError: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := client{}
			err := c.parsePlan(tt.path)

			if tt.expectedError {
				assert.Errorf(t, err, "expected an error when calling parsePlan")
			}
			if !tt.expectedError {
				assert.NoErrorf(t, err, "didn't expect an error when calling parsePlan")
				assert.Equalf(t, tt.expectedResourceChanges, len(c.terraformPlan.ResourceChanges), "unexpected number of resource changes when running parsePlan")
			}
		})
	}
}

func Test_contains(t *testing.T) {
	cases := []struct {
		name          string
		sourceSlice   []string
		searchString  string
		expectedFound bool
	}{
		{name: "expected found", sourceSlice: []string{"t3.micro", "m5.large"}, searchString: "m5.large", expectedFound: true},
		{name: "expected not found", sourceSlice: []string{"t3.micro", "m5.large"}, searchString: "bad.search", expectedFound: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.sourceSlice, tt.searchString)
			if tt.expectedFound {
				assert.True(t, result, "expected the string to be found using contains")
			}

			if !tt.expectedFound {
				assert.False(t, result, "expected the string not to be found")
			}
		})
	}
}

func Test_processSingleChange(t *testing.T) {
	cases := []struct {
		name                string
		planPath            string
		changeIndex         int
		expectedError       bool
		expectedInvalidType bool
		invalidTypeDetails  invalidInstanceType
	}{
		{name: "expected-invalid-type-aws_instance-update", planPath: "./testdata/two-changes-bad.json", changeIndex: 0, expectedError: false, expectedInvalidType: true, invalidTypeDetails: invalidInstanceType{
			address:      "aws_instance.web",
			resourceType: "aws_instance",
			instanceType: "INVALID_TYPE",
		}},
		{name: "expected-invalid-type-aws_launch_template-update", planPath: "./testdata/two-changes-bad.json", changeIndex: 1, expectedError: false, expectedInvalidType: true, invalidTypeDetails: invalidInstanceType{
			address:      "aws_launch_template.web",
			resourceType: "aws_launch_template",
			instanceType: "ANOTHER_BAD_TYPE",
		}},
		{name: "expected-invalid-type-aws_launch_configuration-update", planPath: "./testdata/aws-launch-configuration-create.json", changeIndex: 1, expectedError: false, expectedInvalidType: true, invalidTypeDetails: invalidInstanceType{
			address:      "aws_launch_configuration.web",
			resourceType: "aws_launch_configuration",
			instanceType: "YET_ANOTHER_BAD_TYPE",
		}},
		{name: "unsupported-resource-type", planPath: "./testdata/aws-launch-configuration-create.json", changeIndex: 3, expectedError: false, expectedInvalidType: false, invalidTypeDetails: invalidInstanceType{}},
		{name: "supported-instance-type-aws_instance-create", planPath: "./testdata/aws-instance-create.json", changeIndex: 0, expectedError: false, expectedInvalidType: false, invalidTypeDetails: invalidInstanceType{}},
		{name: "no-op-action", planPath: "./testdata/aws-instance-noop.json", changeIndex: 0, expectedError: false, expectedInvalidType: false, invalidTypeDetails: invalidInstanceType{}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pager := &mockDescribeInstanceTypeOfferingsPaginator{
				Pages: []*ec2.DescribeInstanceTypeOfferingsOutput{
					{InstanceTypeOfferings: []types.InstanceTypeOffering{
						{InstanceType: "t3.micro"},
						{InstanceType: "m5.large"},
						{InstanceType: "c7.medium"},
					}}},
			}

			c := client{paginator: pager, logger: logging.NewLogger("ERROR", os.Stdout)}

			err := c.parsePlan(tt.planPath)
			assert.NoErrorf(t, err, "expected no error when running parsePlan")

			results, err := c.processSingleChange(c.terraformPlan.ResourceChanges[tt.changeIndex])

			if tt.expectedError {
				assert.Errorf(t, err, "expected an error when calling processSingleChange")
			}
			if !tt.expectedError {
				assert.NoErrorf(t, err, "did not expect an error when calling processSingleChange")
			}

			if tt.expectedInvalidType {
				assert.Equal(t, tt.invalidTypeDetails, results, "returned error details not expected")
			}
			if !tt.expectedInvalidType {
				assert.Equal(t, "", results.instanceType)
				assert.Equal(t, "", results.address)
				assert.Equal(t, "", results.resourceType)
			}

		})
	}

}
