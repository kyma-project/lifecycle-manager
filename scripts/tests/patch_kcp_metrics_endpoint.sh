#!/bin/bash

kubectl patch svc klm-controller-manager-metrics -p '{"spec": {"type": "LoadBalancer"}}' -n kcp-system
