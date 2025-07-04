# Provide Credentials for Private OCI Registry Authentication

## Context

If you use a private OCI registry for hosting module resources, follow the instructions to provide your authentication credentials.

## Prerequisites

For a private registry that requires access permissions, each major registry provider offers standard Docker access credentials with a username and password combination.

As an example, Docker Hub lets you create personal [access tokens](https://docs.docker.com/docker-hub/access-tokens/) as an alternative to your password.

Before you proceed, prepare your registry credentials. Check also how to deal with the credentials rotation as is not covered in this guide.

> **TIP:** To comply with the principle of least privilege (PoLP), make sure these credentials have no more than the read-only permissions granted.

## Procedure


1. Create a Docker Registry Secret resource definition:

   ```sh
   kubectl create secret docker-registry [secret name] --docker-server=[your oci registry host] --docker-username=[username] --docker-password=[password/token]  --dry-run=client -oyaml > registry_cred_secret.yaml
   ```

2. Adapt the Deployment of Lifecycle Manager (KLM). Use the flag `--oci-registry-cred-secret` together with the value of your Secret name to ensure that the private OCI registry is used. For example, if your Secret is named `my-private-oci-reg-creds`, your Lifecycle Manager Deployment must contain the following container argument:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: klm-controller-manager
     namespace: kcp-system
   spec:
     template:
       containers:
       - args:
         - oci-registry-cred-secret=my-private-oci-reg-creds
   ```

3. Deploy the Secret in the same cluster where KLM is deployed. Usually, it is the `kcp-system` namespace in the KCP cluster.

   ```sh
   kubectl apply -f registry_cred_secret.yaml -n kcp-system
   ```
