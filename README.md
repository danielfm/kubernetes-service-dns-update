# Kubernetes Service DNS Update

[![Build Status](https://travis-ci.org/danielfm/kubernetes-service-dns-update.svg?branch=master)](https://travis-ci.org/danielfm/kubernetes-service-dns-update)
[![Coverage Status](https://coveralls.io/repos/github/danielfm/kubernetes-service-dns-update/badge.svg?branch=master)](https://coveralls.io/github/danielfm/kubernetes-service-dns-update?branch=master)
[![experimental](http://badges.github.io/stability-badges/dist/experimental.svg)](http://github.com/badges/stability-badges)

This daemon handles the task of synchronizing DNS records - for now, Route53
only - to Kubernetes services.

This is a fork of [route53-kubernetes](https://github.com/wearemolecule/route53-kubernetes)
project. These are the main changes I've made:

- Command line switch for switch dry-run on/off
- Command line argument to specify the namespace to be watched
- Command line to customize the sync interval
- Removed dependency to glog
- Better test coverage

## How it Works

The daemon lists all services configured with the label `dns: route53` (from a
given namespace, or all namespaces) and adds the appropriate aliases to the
domains (top-level domains are also supported) specified by the annotation
`domainNames`.

The daemon must be running inside a Kubernetes node on AWS and that the IAM
profile for that node is set up to allow the following, along with the default
permissions needed by Kubernetes:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "route53:ListHostedZonesByName",
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": "elasticloadbalancing:DescribeLoadBalancers",
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": "route53:ChangeResourceRecordSets",
            "Resource": "*"
        }
    ]
}
```

### Service Configuration

Given the following Kubernetes service definition:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  labels:
    app: my-app
    role: web
    dns: route53
  annotations:
    domainNames: test.mydomain.com
spec:
  selector:
    app: my-app
    role: web
  ports:
  - name: web
    port: 80
    protocol: TCP
    targetPort: web
  - name: web-ssl
    port: 443
    protocol: TCP
    targetPort: web-ssl
  type: LoadBalancer
```

An "A" record for `test.mydomain.com` will be created as an alias to the ELB that
is configured by kubernetes. This assumes that a hosted zone exists in Route53 for
`mydomain.com`. Any record that previously existed for that dns record will be
updated.
