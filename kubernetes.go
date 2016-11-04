package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	"k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/labels"
	"k8s.io/client-go/1.4/rest"
)

type KubernetesClientImpl struct {
	clientset *kubernetes.Clientset
}

type KubernetesClient interface {
	GetDNSServices(namespace, selector string) ([]v1.Service, error)
}

func NewKubernetesClient() (*KubernetesClientImpl, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubernetesClientImpl{
		clientset: clientset,
	}, nil
}

func (c *KubernetesClientImpl) GetDNSServices(namespace, selector string) ([]v1.Service, error) {
	l, err := labels.Parse(selector)
	if err != nil {
		log.Fatalf("Failed to parse selector %q: %v", selector, err)
	}
	opts := api.ListOptions{
		LabelSelector: l,
	}

	services, err := c.clientset.Core().Services(namespace).List(opts)

	if err != nil {
		return nil, err
	}

	return services.Items, nil
}

func ServiceIngressHostname(service v1.Service) (string, error) {
	ingress := service.Status.LoadBalancer.Ingress
	if len(ingress) < 1 {
		return "", errors.New("No ingress defined for ELB")
	}
	if len(ingress) > 1 {
		return "", errors.New("Multiple ingress points found for ELB not supported")
	}
	return ingress[0].Hostname, nil
}

func ServiceDomainNames(service v1.Service) ([]string, error) {
	annotation, ok := service.ObjectMeta.Annotations["domainNames"]
	if !ok {
		return nil, fmt.Errorf("Annotation 'domainNames' not set for %s", service.ObjectMeta.Name)
	}

	domainNames := strings.Split(annotation, ",")
	for i, domainName := range domainNames {
		domainNames[i] = strings.TrimSpace(domainName)
	}

	return domainNames, nil
}
