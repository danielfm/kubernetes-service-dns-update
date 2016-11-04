package main

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/route53"
)

type DummyRoute53Client struct {
	t *testing.T

	listHostedZonesByNameInput  *route53.ListHostedZonesByNameInput
	listHostedZonesByNameOutput *route53.ListHostedZonesByNameOutput
	listHostedZonesByNameError  error

	changeResourceRecordSetsInput *route53.ChangeResourceRecordSetsInput
	changeResourceRecordSetsError error
}

type DummyELBClient struct {
	t *testing.T

	describeLoadBalancersInput  *elb.DescribeLoadBalancersInput
	describeLoadBalancersOutput *elb.DescribeLoadBalancersOutput
	describeLoadBalancersError  error
}

func (c DummyRoute53Client) ListHostedZonesByName(input *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	expectedInput := awsutil.StringValue(c.listHostedZonesByNameInput)
	actualInput := awsutil.StringValue(input)

	if expectedInput != actualInput {
		c.t.Errorf("Expected input to be '%s', was '%s'", expectedInput, actualInput)
	}

	return c.listHostedZonesByNameOutput, c.listHostedZonesByNameError
}

func (c DummyRoute53Client) ChangeResourceRecordSets(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	expectedInput := awsutil.StringValue(c.changeResourceRecordSetsInput)
	actualInput := awsutil.StringValue(input)

	if expectedInput != actualInput {
		c.t.Errorf("Expected input to be '%s', was '%s'", expectedInput, actualInput)
	}

	return nil, c.changeResourceRecordSetsError
}

func (c DummyELBClient) DescribeLoadBalancers(input *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
	expectedInput := awsutil.StringValue(c.describeLoadBalancersInput)
	actualInput := awsutil.StringValue(input)

	if expectedInput != actualInput {
		c.t.Errorf("Expected input to be '%s', was '%s'", expectedInput, actualInput)
	}

	return c.describeLoadBalancersOutput, c.describeLoadBalancersError
}

func TestGetHostedZoneID(t *testing.T) {
	scenarios := []struct {
		domain string

		listHostedZonesByNameInput  *route53.ListHostedZonesByNameInput
		listHostedZonesByNameOutput *route53.ListHostedZonesByNameOutput
		listHostedZonesByNameError  error

		expectedHostedZoneID string
		expectedError        error
	}{
		// Valid destination zone
		{
			domain: "valid.domain.com",

			listHostedZonesByNameInput: &route53.ListHostedZonesByNameInput{
				DNSName: aws.String("domain.com"),
			},
			listHostedZonesByNameOutput: &route53.ListHostedZonesByNameOutput{
				HostedZones: []*route53.HostedZone{
					&route53.HostedZone{
						Name: aws.String("domain.com."),
						Id:   aws.String("/hostedzone/ABC123"),
					},
				},
			},
			listHostedZonesByNameError: nil,

			expectedHostedZoneID: "ABC123",
			expectedError:        nil,
		},

		// No zone found
		{
			domain: "valid.domain.com",

			listHostedZonesByNameInput: &route53.ListHostedZonesByNameInput{
				DNSName: aws.String("domain.com"),
			},
			listHostedZonesByNameOutput: nil,
			listHostedZonesByNameError:  errors.New("error"),

			expectedHostedZoneID: "",
			expectedError:        errors.New("No zone found for domain.com: error"),
		},

		// Invalid domain according to getTLD
		{
			domain: "top-level-domain.com",

			expectedHostedZoneID: "",
			expectedError:        errors.New("Domain top-level-domain.com is invalid - it should be a fully qualified domain name and subdomain (i.e. test.example.com)"),
		},

		// Invalid domain according to findMostSpecificZoneForDomain (sanity check)
		{
			domain: "valid.domain.com",

			listHostedZonesByNameInput: &route53.ListHostedZonesByNameInput{
				DNSName: aws.String("domain.com"),
			},
			listHostedZonesByNameOutput: &route53.ListHostedZonesByNameOutput{
				HostedZones: []*route53.HostedZone{
					&route53.HostedZone{
						Name: aws.String("other.com."),
						Id:   aws.String("/hostedzone/ABC123"),
					},
				},
			},
			listHostedZonesByNameError: nil,

			expectedHostedZoneID: "",
			expectedError:        errors.New("No zone matches domain valid.domain.com."),
		},
	}

	for _, scenario := range scenarios {
		awsClient := &AWSClientImpl{
			route53: &DummyRoute53Client{
				t: t,

				listHostedZonesByNameInput:  scenario.listHostedZonesByNameInput,
				listHostedZonesByNameOutput: scenario.listHostedZonesByNameOutput,
				listHostedZonesByNameError:  scenario.listHostedZonesByNameError,
			},
		}

		hostedZoneID, err := awsClient.GetHostedZoneID(scenario.domain)

		if err != nil && err.Error() != scenario.expectedError.Error() {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		} else if hostedZoneID != scenario.expectedHostedZoneID {
			t.Errorf("Expected hosted zone to be '%s', was '%s'", scenario.expectedHostedZoneID, hostedZoneID)
		}
	}
}

func TestGetLoadBalancerHostedZoneID(t *testing.T) {
	scenarios := []struct {
		hostname string

		describeLoadBalancersInput  *elb.DescribeLoadBalancersInput
		describeLoadBalancersOutput *elb.DescribeLoadBalancersOutput
		describeLoadBalancersError  error

		expectedZoneID string
		expectedError  error
	}{
		// Invalid load balancer name
		{
			hostname: "invalid.elb.amazonaws.com",

			expectedZoneID: "",
			expectedError:  errors.New("Could not parse ELB hostname: invalid.elb.amazonaws.com is not a valid ELB hostname"),
		},

		// No load balancers found
		{
			hostname: "testpublic-1111111111.us-east-1.elb.amazonaws.com",

			describeLoadBalancersInput: &elb.DescribeLoadBalancersInput{
				LoadBalancerNames: []*string{
					aws.String("testpublic"),
				},
			},
			describeLoadBalancersOutput: &elb.DescribeLoadBalancersOutput{
				LoadBalancerDescriptions: []*elb.LoadBalancerDescription{},
			},
			describeLoadBalancersError: nil,

			expectedZoneID: "",
			expectedError:  errors.New("No load balancer found"),
		},

		// Error when trying to describe the load balancers
		{
			hostname: "testpublic-1111111111.us-east-1.elb.amazonaws.com",

			describeLoadBalancersInput: &elb.DescribeLoadBalancersInput{
				LoadBalancerNames: []*string{
					aws.String("testpublic"),
				},
			},
			describeLoadBalancersOutput: nil,
			describeLoadBalancersError:  errors.New("error"),

			expectedZoneID: "",
			expectedError:  errors.New("Could not describe load balancer: error"),
		},

		// More than one load balancer found
		{
			hostname: "testpublic-1111111111.us-east-1.elb.amazonaws.com",

			describeLoadBalancersInput: &elb.DescribeLoadBalancersInput{
				LoadBalancerNames: []*string{
					aws.String("testpublic"),
				},
			},
			describeLoadBalancersOutput: &elb.DescribeLoadBalancersOutput{
				LoadBalancerDescriptions: []*elb.LoadBalancerDescription{
					&elb.LoadBalancerDescription{},
					&elb.LoadBalancerDescription{},
				},
			},
			describeLoadBalancersError: nil,

			expectedZoneID: "",
			expectedError:  errors.New("Multiple load balancers found"),
		},

		// One load balancer found
		{
			hostname: "testpublic-1111111111.us-east-1.elb.amazonaws.com",

			describeLoadBalancersInput: &elb.DescribeLoadBalancersInput{
				LoadBalancerNames: []*string{
					aws.String("testpublic"),
				},
			},
			describeLoadBalancersOutput: &elb.DescribeLoadBalancersOutput{
				LoadBalancerDescriptions: []*elb.LoadBalancerDescription{
					&elb.LoadBalancerDescription{
						CanonicalHostedZoneNameID: aws.String("DOMAINZONEID"),
					},
				},
			},
			describeLoadBalancersError: nil,

			expectedZoneID: "DOMAINZONEID",
			expectedError:  nil,
		},
	}

	for _, scenario := range scenarios {
		awsClient := &AWSClientImpl{
			elb: &DummyELBClient{
				t: t,

				describeLoadBalancersInput:  scenario.describeLoadBalancersInput,
				describeLoadBalancersOutput: scenario.describeLoadBalancersOutput,
				describeLoadBalancersError:  scenario.describeLoadBalancersError,
			},
		}

		zoneID, err := awsClient.GetLoadBalancerHostedZoneID(scenario.hostname)

		if err != nil && err.Error() != scenario.expectedError.Error() {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		} else if zoneID != scenario.expectedZoneID {
			t.Errorf("Expected hosted zone to be '%s', was '%s'", scenario.expectedZoneID, zoneID)
		}
	}
}

func TestUpdateDNS(t *testing.T) {
	scenarios := []struct {
		elbHostname          string
		elbHostedZoneID      string
		domainHostedZoneName string
		domainHostedZoneID   string

		changeResourceRecordSetsInput *route53.ChangeResourceRecordSetsInput

		expectedError error
	}{
		// Successful update for subdomain
		{
			elbHostname:          "testpublic-1111111111.us-east-1.elb.amazonaws.com",
			elbHostedZoneID:      "ELB123",
			domainHostedZoneName: "test.domain.com",
			domainHostedZoneID:   "DNS123",

			changeResourceRecordSetsInput: &route53.ChangeResourceRecordSetsInput{
				ChangeBatch: &route53.ChangeBatch{
					Changes: []*route53.Change{
						&route53.Change{
							Action: aws.String("UPSERT"),
							ResourceRecordSet: &route53.ResourceRecordSet{
								AliasTarget: &route53.AliasTarget{
									DNSName:              aws.String("dualstack.testpublic-1111111111.us-east-1.elb.amazonaws.com"),
									EvaluateTargetHealth: aws.Bool(false),
									HostedZoneId:         aws.String("ELB123"),
								},
								Name: aws.String("test.domain.com"),
								Type: aws.String("A"),
							},
						},
					},
					Comment: aws.String("Kubernetes Update to Service"),
				},
				HostedZoneId: aws.String("DNS123"),
			},

			expectedError: nil,
		},

		// Successful update for top-level domain
		{
			elbHostname:          "testpublic-1111111111.us-east-1.elb.amazonaws.com",
			elbHostedZoneID:      "ELB123",
			domainHostedZoneName: ".domain.com",
			domainHostedZoneID:   "DNS123",

			changeResourceRecordSetsInput: &route53.ChangeResourceRecordSetsInput{
				ChangeBatch: &route53.ChangeBatch{
					Changes: []*route53.Change{
						&route53.Change{
							Action: aws.String("UPSERT"),
							ResourceRecordSet: &route53.ResourceRecordSet{
								AliasTarget: &route53.AliasTarget{
									DNSName:              aws.String("dualstack.testpublic-1111111111.us-east-1.elb.amazonaws.com"),
									EvaluateTargetHealth: aws.Bool(false),
									HostedZoneId:         aws.String("ELB123"),
								},
								Name: aws.String("domain.com"),
								Type: aws.String("A"),
							},
						},
					},
					Comment: aws.String("Kubernetes Update to Service"),
				},
				HostedZoneId: aws.String("DNS123"),
			},

			expectedError: nil,
		},

		// Failed update
		{
			elbHostname:          "testpublic-1111111111.us-east-1.elb.amazonaws.com",
			elbHostedZoneID:      "ELB123",
			domainHostedZoneName: "test.domain.com",
			domainHostedZoneID:   "DNS123",

			changeResourceRecordSetsInput: &route53.ChangeResourceRecordSetsInput{
				ChangeBatch: &route53.ChangeBatch{
					Changes: []*route53.Change{
						&route53.Change{
							Action: aws.String("UPSERT"),
							ResourceRecordSet: &route53.ResourceRecordSet{
								AliasTarget: &route53.AliasTarget{
									DNSName:              aws.String("dualstack.testpublic-1111111111.us-east-1.elb.amazonaws.com"),
									EvaluateTargetHealth: aws.Bool(false),
									HostedZoneId:         aws.String("ELB123"),
								},
								Name: aws.String("test.domain.com"),
								Type: aws.String("A"),
							},
						},
					},
					Comment: aws.String("Kubernetes Update to Service"),
				},
				HostedZoneId: aws.String("DNS123"),
			},

			expectedError: errors.New("error"),
		},
	}

	for _, scenario := range scenarios {
		awsClient := &AWSClientImpl{
			route53: &DummyRoute53Client{
				t: t,

				changeResourceRecordSetsInput: scenario.changeResourceRecordSetsInput,
				changeResourceRecordSetsError: scenario.expectedError,
			},
		}

		err := awsClient.UpdateDNS(scenario.elbHostname, scenario.elbHostedZoneID, scenario.domainHostedZoneName, scenario.domainHostedZoneID)

		if scenario.expectedError != nil && err.Error() != scenario.expectedError.Error() {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		}
	}
}

func TestLoadBalancerNameFromHostname(t *testing.T) {
	scenarios := map[string]string{
		"testpublic-1111111111.us-east-1.elb.amazonaws.com":            "testpublic",
		"internal-testinternal-2222222222.us-east-1.elb.amazonaws.com": "testinternal",
	}

	for hostname, elbName := range scenarios {
		extractedName, err := loadBalancerNameFromHostname(hostname)
		if err != nil {
			t.Errorf("Expected %s to parse to %s, but got %v", hostname, elbName, err)
		}
		if extractedName != elbName {
			t.Errorf("Expected %s but got %s for hostname %s", elbName, extractedName, hostname)
		}
	}

	invalid := []string{
		"nodashes",
		"internal",
	}

	for _, bad := range invalid {
		extractedName, err := loadBalancerNameFromHostname(bad)
		if err == nil {
			t.Errorf("Expected %s to parse to fail, but got %v", bad, extractedName)
		}
	}
}

func TestFinMostSpecificZoneForDomainWithInvalidInput(t *testing.T) {
	demo := route53.HostedZone{
		Name: aws.String("demo.com."),
	}
	demoSub := route53.HostedZone{
		Name: aws.String("sub.demo.com."),
	}
	zones := []*route53.HostedZone{
		&demo,
		&demoSub,
	}

	actualZone, err := findMostSpecificZoneForDomain(".demo.com", []*route53.HostedZone{})
	if err == nil {
		t.Error("Expected error to be raised, but returned", actualZone)
		return
	}

	actualZone, err = findMostSpecificZoneForDomain("test.other.com", zones)
	if err == nil {
		t.Error("Expected error to be raised, but returned", actualZone)
		return
	}
}

func TestFindMostSpecificZoneForDomain(t *testing.T) {
	demo := route53.HostedZone{
		Name: aws.String("demo.com."),
	}
	demoSub := route53.HostedZone{
		Name: aws.String("sub.demo.com."),
	}
	zones := []*route53.HostedZone{
		&demo,
		&demoSub,
	}

	scenarios := map[string]*route53.HostedZone{
		".demo.com":           &demo,
		"test.demo.com":       &demo,
		"test.again.demo.com": &demo,
		"sub.demo.com":        &demoSub,
		"test.sub.demo.com":   &demoSub,
	}

	for domain, expectedZone := range scenarios {
		actualZone, err := findMostSpecificZoneForDomain(domain, zones)
		if err != nil {
			t.Error("Expected no error to be raised", err)
			return
		}
		if actualZone != expectedZone {
			t.Errorf("Expected %s to eq %s for domain %s", *actualZone, *expectedZone, domain)
		}
	}

}

func TestDomainWithTrailingDot(t *testing.T) {
	scenarios := map[string]string{
		".test.com":         ".test.com.",
		"hello.goodbye.io.": "hello.goodbye.io.",
	}

	for withoutDot, withDot := range scenarios {
		result := domainWithTrailingDot(withoutDot)
		if result != withDot {
			t.Errorf("Expected %s but got %s for hostname %s", withDot, result, withoutDot)
		}
	}
}

func TestGetTLD(t *testing.T) {
	scenarios := map[string]string{
		".test.com":                            "test.com",
		"hello.goodbye.io":                     "goodbye.io",
		"this.is.really.long.hello.goodbye.io": "goodbye.io",
	}

	for domain, tld := range scenarios {
		result, err := getTLD(domain)
		if err != nil {
			t.Error("Unexpected error: ", err)
		}
		if result != tld {
			t.Errorf("Expected %s but got %s for tld %s", tld, result, domain)
		}
	}

	bad := "bad.domain"
	_, err := getTLD(bad)
	if err == nil {
		t.Errorf("%s should cause error", bad)
	}
}
