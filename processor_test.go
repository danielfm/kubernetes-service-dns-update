package main

import (
	"errors"
	"testing"

	"k8s.io/client-go/1.4/pkg/api/v1"
)

type KubernetesClientDummy struct {
	t *testing.T

	getDNSServicesSelector string
	getDNSServicesOutput   []v1.Service
	getDNSServicesError    error
}

type AWSClientDummy struct {
	t *testing.T

	getHostedZoneIDDomain string
	getHostedZoneIDOutput string
	getHostedZoneIDError  error

	getLoadBalancerHostedZoneIDHostname string
	getLoadBalancerHostedZoneIDOutput   string
	getLoadBalancerHostedZoneIDError    error

	updateDNSELBHostname        string
	updateDNSELBHostedZoneID    string
	updateDNSDomainName         string
	updateDNSDomainHostedZoneID string
	updateDNSError              error
}

func (c KubernetesClientDummy) GetDNSServices(ns, selector string) ([]v1.Service, error) {
	if ns != namespace {
		c.t.Errorf("Expected namespace to be '%s', was '%s'", namespace, ns)
	}

	if selector != c.getDNSServicesSelector {
		c.t.Errorf("Expected namespace to be '%s', was '%s'", namespace, ns)
	}

	return c.getDNSServicesOutput, c.getDNSServicesError
}

func (c AWSClientDummy) GetHostedZoneID(domain string) (string, error) {
	if domain != c.getHostedZoneIDDomain {
		c.t.Errorf("Expected domain to be '%s', was '%s'", c.getHostedZoneIDDomain, domain)
	}

	return c.getHostedZoneIDOutput, c.getHostedZoneIDError
}

func (c AWSClientDummy) GetLoadBalancerHostedZoneID(hostname string) (string, error) {
	if hostname != c.getLoadBalancerHostedZoneIDHostname {
		c.t.Errorf("Expected hostname to be '%s', was '%s'", c.getLoadBalancerHostedZoneIDHostname, hostname)
	}

	return c.getLoadBalancerHostedZoneIDOutput, c.getLoadBalancerHostedZoneIDError
}

func (c AWSClientDummy) UpdateDNS(elbHostname, elbHostedZoneID, domainName, domainHostedZoneID string) error {
	if elbHostname != c.updateDNSELBHostname {
		c.t.Errorf("Expected elbHostname to be '%s', was '%s'", c.updateDNSELBHostname, elbHostname)
	}

	if elbHostedZoneID != c.updateDNSELBHostedZoneID {
		c.t.Errorf("Expected elbHostedZoneID to be '%s', was '%s'", c.updateDNSELBHostedZoneID, elbHostedZoneID)
	}

	if domainName != c.updateDNSDomainName {
		c.t.Errorf("Expected domainName to be '%s', was '%s'", c.updateDNSDomainName, domainName)
	}

	if domainHostedZoneID != c.updateDNSDomainHostedZoneID {
		c.t.Errorf("Expected domainHostedZoneID to be '%s', was '%s'", c.updateDNSDomainHostedZoneID, domainHostedZoneID)
	}

	return c.updateDNSError
}

func TestSyncRoute53DNSRecords(t *testing.T) {
	scenarios := []struct {
		getDNSServicesSelector string
		getDNSServicesOutput   []v1.Service
		getDNSServicesError    error

		getHostedZoneIDDomain string
		getHostedZoneIDOutput string
		getHostedZoneIDError  error

		getLoadBalancerHostedZoneIDHostname string
		getLoadBalancerHostedZoneIDOutput   string
		getLoadBalancerHostedZoneIDError    error

		updateDNSELBHostname        string
		updateDNSELBHostedZoneID    string
		updateDNSDomainName         string
		updateDNSDomainHostedZoneID string
		updateDNSError              error

		expectedError error
	}{
		// Error while trying to fetch services from Kubernetes
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesError:    errors.New("error"),

			expectedError: errors.New("Failed to list pods: error"),
		},

		// Service without load balancer ingress hostname
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					ObjectMeta: v1.ObjectMeta{
						Name:      "service",
						Namespace: namespace,
					},
				},
			},

			expectedError: nil,
		},

		// Service without domainNames annotation
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								v1.LoadBalancerIngress{
									Hostname: "elb.hostname.amazonaws.com",
								},
							},
						},
					},
				},
			},

			expectedError: nil,
		},

		// Error trying to retrieve load balancer hosted zone ID
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					ObjectMeta: v1.ObjectMeta{
						Name:        "service",
						Namespace:   namespace,
						Annotations: map[string]string{"domainNames": "some.domain.com"},
					},
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								v1.LoadBalancerIngress{
									Hostname: "elb.hostname.amazonaws.com",
								},
							},
						},
					},
				},
			},

			getLoadBalancerHostedZoneIDHostname: "elb.hostname.amazonaws.com",
			getLoadBalancerHostedZoneIDError:    errors.New("error"),

			expectedError: nil,
		},

		// Error trying to retrieve custom domain hosted zone ID
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					ObjectMeta: v1.ObjectMeta{
						Name:        "service",
						Namespace:   namespace,
						Annotations: map[string]string{"domainNames": "some.domain.com"},
					},
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								v1.LoadBalancerIngress{
									Hostname: "elb.hostname.amazonaws.com",
								},
							},
						},
					},
				},
			},

			getLoadBalancerHostedZoneIDHostname: "elb.hostname.amazonaws.com",
			getLoadBalancerHostedZoneIDOutput:   "ELBZONEID",

			getHostedZoneIDDomain: "some.domain.com",
			getHostedZoneIDError:  errors.New("error"),

			expectedError: nil,
		},

		// Error trying to update DNS records
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					ObjectMeta: v1.ObjectMeta{
						Name:        "service",
						Namespace:   namespace,
						Annotations: map[string]string{"domainNames": "some.domain.com"},
					},
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								v1.LoadBalancerIngress{
									Hostname: "elb.hostname.amazonaws.com",
								},
							},
						},
					},
				},
			},

			getLoadBalancerHostedZoneIDHostname: "elb.hostname.amazonaws.com",
			getLoadBalancerHostedZoneIDOutput:   "ELBZONEID",

			getHostedZoneIDDomain: "some.domain.com",
			getHostedZoneIDOutput: "DOMAINZONEID",

			updateDNSELBHostname:        "elb.hostname.amazonaws.com",
			updateDNSELBHostedZoneID:    "ELBZONEID",
			updateDNSDomainName:         "some.domain.com",
			updateDNSDomainHostedZoneID: "DOMAINZONEID",
			updateDNSError:              errors.New("error"),

			expectedError: nil,
		},

		// Successful update
		{
			getDNSServicesSelector: "dns=route53",
			getDNSServicesOutput: []v1.Service{
				v1.Service{
					ObjectMeta: v1.ObjectMeta{
						Name:        "service",
						Namespace:   namespace,
						Annotations: map[string]string{"domainNames": "some.domain.com"},
					},
					Status: v1.ServiceStatus{
						LoadBalancer: v1.LoadBalancerStatus{
							Ingress: []v1.LoadBalancerIngress{
								v1.LoadBalancerIngress{
									Hostname: "elb.hostname.amazonaws.com",
								},
							},
						},
					},
				},
			},

			getLoadBalancerHostedZoneIDHostname: "elb.hostname.amazonaws.com",
			getLoadBalancerHostedZoneIDOutput:   "ELBZONEID",

			getHostedZoneIDDomain: "some.domain.com",
			getHostedZoneIDOutput: "DOMAINZONEID",

			updateDNSELBHostname:        "elb.hostname.amazonaws.com",
			updateDNSELBHostedZoneID:    "ELBZONEID",
			updateDNSDomainName:         "some.domain.com",
			updateDNSDomainHostedZoneID: "DOMAINZONEID",
			updateDNSError:              nil,

			expectedError: nil,
		},
	}

	for _, scenario := range scenarios {
		kubernetesClient := KubernetesClientDummy{
			t: t,

			getDNSServicesSelector: scenario.getDNSServicesSelector,
			getDNSServicesOutput:   scenario.getDNSServicesOutput,
			getDNSServicesError:    scenario.getDNSServicesError,
		}

		awsClient := AWSClientDummy{
			t: t,

			getLoadBalancerHostedZoneIDHostname: scenario.getLoadBalancerHostedZoneIDHostname,
			getLoadBalancerHostedZoneIDOutput:   scenario.getLoadBalancerHostedZoneIDOutput,
			getLoadBalancerHostedZoneIDError:    scenario.getLoadBalancerHostedZoneIDError,

			getHostedZoneIDDomain: scenario.getHostedZoneIDDomain,
			getHostedZoneIDOutput: scenario.getHostedZoneIDOutput,
			getHostedZoneIDError:  scenario.getHostedZoneIDError,

			updateDNSELBHostname:        scenario.updateDNSELBHostname,
			updateDNSELBHostedZoneID:    scenario.updateDNSELBHostedZoneID,
			updateDNSDomainName:         scenario.updateDNSDomainName,
			updateDNSDomainHostedZoneID: scenario.updateDNSDomainHostedZoneID,
			updateDNSError:              scenario.updateDNSError,
		}

		err := SyncRoute53DNSRecords(kubernetesClient, awsClient)

		if (err == nil && scenario.expectedError != nil) || (scenario.expectedError != nil && scenario.expectedError.Error() != err.Error()) {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		}
	}
}
