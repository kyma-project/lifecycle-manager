# Architecture decisions

## Deployment models
Kyma-operator (and component operators) can run in following modes:
- in-cluster - regular deployment in the kubernetes cluster where kyma should be deployed
- control-plane - deployment on central kubernetes cluster that manages multiple kyma installations (installing kyma on the remote clusters)
- cli - local binary using kubeconfig to install kyma components on target cluster

## Separate repositories for component operators
Teams providing component operators should work (and release) independently from main kyma-operator and other component operators. 

