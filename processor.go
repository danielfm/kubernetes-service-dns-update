package main

import (
	"fmt"
	"log"
	"sync"
	"time"
)

func WatchServices(interval int, done chan struct{}, wg *sync.WaitGroup) {
	go func() {
		kubernetesClient, err := NewKubernetesClient()
		if err != nil {
			panic(err.Error())
		}

		awsClient, err := NewAWSClient()
		if err != nil {
			panic(err.Error())
		}

		for {
			select {
			case <-time.After(time.Duration(interval) * time.Second):
				err := SyncRoute53DNSRecords(kubernetesClient, awsClient)
				if err != nil {
					log.Println(err)
				}
			case <-done:
				wg.Done()
				log.Println("Stopped DNS update service.")
				return
			}
		}
	}()
}

func SyncRoute53DNSRecords(kubernetesClient KubernetesClient, awsClient AWSClient) error {
	selector := "dns=route53"

	services, err := kubernetesClient.GetDNSServices(namespace, selector)
	if err != nil {
		return fmt.Errorf("Failed to list pods: %v", err)
	}

	log.Printf("Found %d DNS services with selector %q\n", len(services), selector)

	for _, service := range services {
		elbHostname, err := ServiceIngressHostname(service)
		if err != nil {
			log.Printf("Could not find ingress hostname for %s: %s\n", service.Name, err)
			continue
		}

		domainNames, err := ServiceDomainNames(service)
		if err != nil {
			log.Println(err)
			continue
		}

		elbHostedZoneID, err := awsClient.GetLoadBalancerHostedZoneID(elbHostname)
		if err != nil {
			log.Printf("Could not get zone ID: %s\n", err)
			continue
		}

		for _, domainName := range domainNames {
			log.Printf("Creating DNS for %s service (%s): %s -> %s\n", service.Name, service.ObjectMeta.Namespace, elbHostname, domainName)

			domainHostedZoneID, err := awsClient.GetHostedZoneID(domainName)
			if err != nil {
				log.Printf("Could not find hosted zone: %s\n", err)
				continue
			}

			if err = awsClient.UpdateDNS(elbHostname, elbHostedZoneID, domainName, domainHostedZoneID); err != nil {
				log.Printf("Failed to update record set: %v\n", err)
				continue
			}

			log.Printf("Created DNS record set: domainName=%s, hostedZoneID=%s\n", domainName, domainHostedZoneID)
		}
	}

	return nil
}
