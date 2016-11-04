package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/route53"
)

type AWSClientImpl struct {
	route53 Route53Client
	elb     ELBClient
}

type AWSClient interface {
	GetHostedZoneID(domain string) (string, error)
	GetLoadBalancerHostedZoneID(hostname string) (string, error)
	UpdateDNS(elbHostname, elbHostedZoneID, domainName, domainHostedZoneID string) error
}

type Route53Client interface {
	ListHostedZonesByName(input *route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error)
	ChangeResourceRecordSets(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error)
}

type ELBClient interface {
	DescribeLoadBalancers(input *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
}

func NewAWSClient() (*AWSClientImpl, error) {
	metadata := ec2metadata.New(session.New())

	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{},
			&ec2rolecreds.EC2RoleProvider{Client: metadata},
		})

	region, err := metadata.Region()
	if err != nil {
		return nil, err
	}

	awsConfig := aws.NewConfig()
	awsConfig.WithCredentials(creds)
	awsConfig.WithRegion(region)

	sess := session.New(awsConfig)

	return &AWSClientImpl{
		route53: route53.New(sess),
		elb:     elb.New(sess),
	}, nil
}

func (c *AWSClientImpl) GetHostedZoneID(domain string) (string, error) {
	tld, err := getTLD(domain)
	if err != nil {
		return "", err
	}

	listHostedZoneInput := route53.ListHostedZonesByNameInput{
		DNSName: &tld,
	}

	hzOut, err := c.route53.ListHostedZonesByName(&listHostedZoneInput)
	if err != nil {
		return "", fmt.Errorf("No zone found for %s: %v", tld, err)
	}

	// TODO: The AWS API may return multiple pages, we should parse them all

	hostedZone, err := findMostSpecificZoneForDomain(domain, hzOut.HostedZones)
	if err != nil {
		return "", err
	}

	zoneParts := strings.Split(aws.StringValue(hostedZone.Id), "/")
	zoneId := zoneParts[len(zoneParts)-1]

	return zoneId, nil
}

func (c *AWSClientImpl) GetLoadBalancerHostedZoneID(hostname string) (string, error) {
	elbName, err := loadBalancerNameFromHostname(hostname)
	if err != nil {
		return "", fmt.Errorf("Could not parse ELB hostname: %v", err)
	}

	lbInput := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			&elbName,
		},
	}

	resp, err := c.elb.DescribeLoadBalancers(lbInput)
	if err != nil {
		return "", fmt.Errorf("Could not describe load balancer: %v", err)
	}

	descs := resp.LoadBalancerDescriptions
	if len(descs) < 1 {
		return "", fmt.Errorf("No load balancer found")
	}
	if len(descs) > 1 {
		return "", fmt.Errorf("Multiple load balancers found")
	}
	return aws.StringValue(descs[0].CanonicalHostedZoneNameID), nil
}

func (c *AWSClientImpl) UpdateDNS(elbHostname, elbHostedZoneID, domainName, domainHostedZoneID string) error {
	crrsInput := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				&route53.Change{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						AliasTarget: &route53.AliasTarget{
							DNSName:              aws.String("dualstack." + elbHostname),
							EvaluateTargetHealth: aws.Bool(false),
							HostedZoneId:         aws.String(elbHostedZoneID),
						},
						Name: aws.String(strings.TrimLeft(domainName, ".")),
						Type: aws.String("A"),
					},
				},
			},
			Comment: aws.String("Kubernetes Update to Service"),
		},
		HostedZoneId: aws.String(domainHostedZoneID),
	}

	if dryRun {
		log.Printf("DRY RUN: We normally would have updated %s to point to %s (%s)\n", domainHostedZoneID, elbHostedZoneID, elbHostname)
		return nil
	}

	_, err := c.route53.ChangeResourceRecordSets(crrsInput)

	return err
}

func getTLD(domain string) (string, error) {
	domainParts := strings.Split(domain, ".")
	segments := len(domainParts)

	if segments < 3 {
		return "", fmt.Errorf("Domain %s is invalid - it should be a fully qualified domain name and subdomain (i.e. test.example.com)", domain)
	}

	return strings.Join(domainParts[segments-2:], "."), nil
}

func domainWithTrailingDot(withoutDot string) string {
	if withoutDot[len(withoutDot)-1:] == "." {
		return withoutDot
	}

	return fmt.Sprint(withoutDot, ".")
}

func findMostSpecificZoneForDomain(domain string, zones []*route53.HostedZone) (*route53.HostedZone, error) {
	domain = domainWithTrailingDot(domain)
	if len(zones) < 1 {
		return nil, fmt.Errorf("No zone found for %s", domain)
	}

	var mostSpecific *route53.HostedZone
	curLen := 0

	for _, zone := range zones {
		zoneName := aws.StringValue(zone.Name)

		if strings.HasSuffix(domain, zoneName) && curLen < len(zoneName) {
			curLen = len(zoneName)
			mostSpecific = zone
		}
	}

	if mostSpecific == nil {
		return nil, fmt.Errorf("No zone matches domain %s", domain)
	}

	return mostSpecific, nil
}

func loadBalancerNameFromHostname(hostname string) (string, error) {
	var name string
	hostnameSegments := strings.Split(hostname, "-")

	if len(hostnameSegments) < 2 {
		return "", fmt.Errorf("%s is not a valid ELB hostname", hostname)
	}

	name = hostnameSegments[0]

	// handle internal load balancer naming
	if name == "internal" {
		name = hostnameSegments[1]
	}

	return name, nil
}
