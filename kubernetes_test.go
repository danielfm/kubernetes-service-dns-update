package main

import (
	"errors"
	"testing"

	"k8s.io/client-go/1.4/pkg/api/v1"
)

func TestServiceHostname(t *testing.T) {
	scenarios := []struct {
		ingress []v1.LoadBalancerIngress

		expectedHostname string
		expectedError    error
	}{
		// No ingress hostnames
		{
			ingress: []v1.LoadBalancerIngress{},

			expectedHostname: "",
			expectedError:    errors.New("No ingress defined for ELB"),
		},

		// One ingress hostname
		{
			ingress: []v1.LoadBalancerIngress{
				v1.LoadBalancerIngress{
					Hostname: "elb.hostname.amazonaws.com",
				},
			},

			expectedHostname: "elb.hostname.amazonaws.com",
			expectedError:    nil,
		},

		// Multiple ingress hostnames
		{
			ingress: []v1.LoadBalancerIngress{
				v1.LoadBalancerIngress{},
				v1.LoadBalancerIngress{},
			},

			expectedHostname: "",
			expectedError:    errors.New("Multiple ingress points found for ELB not supported"),
		},
	}

	for _, scenario := range scenarios {
		service := v1.Service{
			Status: v1.ServiceStatus{
				LoadBalancer: v1.LoadBalancerStatus{
					Ingress: scenario.ingress,
				},
			},
		}

		hostname, err := ServiceIngressHostname(service)

		if err != nil && err.Error() != scenario.expectedError.Error() {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		} else if hostname != scenario.expectedHostname {
			t.Errorf("Expected hostname to be '%v', was '%v'", scenario.expectedHostname, hostname)
		}
	}
}

func TestServiceDomainNames(t *testing.T) {
	scenarios := []struct {
		annotations map[string]string

		expectedDomainNames []string
		expectedError       error
	}{
		// No domains
		{
			annotations: map[string]string{"otherAnnotation": "value"},

			expectedDomainNames: []string{},
			expectedError:       errors.New("Annotation 'domainNames' not set for service"),
		},

		// Single domain
		{
			annotations: map[string]string{"domainNames": "some.domain.com"},

			expectedDomainNames: []string{"some.domain.com"},
			expectedError:       nil,
		},

		// Multiple domains
		{
			annotations: map[string]string{"domainNames": "some.domain.com, other.domain.com"},

			expectedDomainNames: []string{"some.domain.com", "other.domain.com"},
			expectedError:       nil,
		},
	}

	for _, scenario := range scenarios {
		service := v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name:        "service",
				Annotations: scenario.annotations,
			},
		}

		domainNames, err := ServiceDomainNames(service)

		if err != nil && err.Error() != scenario.expectedError.Error() {
			t.Errorf("Expected error to be '%v', was '%v'", scenario.expectedError, err)
		} else {
			if len(scenario.expectedDomainNames) != len(domainNames) {
				t.Errorf("Expected domain names to contain %d items, contains %d", len(scenario.expectedDomainNames), len(domainNames))
			}

			for i, domain := range domainNames {
				if domain != scenario.expectedDomainNames[i] {
					t.Errorf("Expected domain names to be '%v', was '%v'", scenario.expectedDomainNames, domainNames)
				}
			}
		}
	}
}
