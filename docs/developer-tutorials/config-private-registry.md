# Provide credentials for private OCI registry authentication

## Context

If you use a private OCI registry for hosting module resources, follow the instructions to provide your authentication credentials.

## Prerequisites

For a private registry that requires access permissions, each major registry provider offers standard Docker access credentials with a username and password combination.

As an example, Docker Hub lets you create personal [access tokens](https://docs.docker.com/docker-hub/access-tokens/) as an alternative to your password.

Before you proceed, prepare your registry credentials. Check also how to deal with the credentials rotation as is not covered in this guide.

> **TIP:** To comply with the principle of least privilege (PoLP), make sure these credentials have no more than the read-only permissions granted.

## Procedure

### Prepare a docker-registry Secret manifest

1. Create a docker-registry Secret manifest. Run:

   ```sh
   kubectl create secret docker-registry [secret name] --docker-server=[your oci registry host] --docker-username=[username] --docker-password=[password/token]  --dry-run=client -oyaml > registry_cred_secret.yaml
   ```

2. Add the following labels to the Secret so that it can be configured later in the ModuleTemplate custom resource (CR) as a label selector.

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
      labels:
        "operator.kyma-project.io/managed-by": "lifecycle-manager"
        "operator.kyma-project.io/oci-registry-cred": "test-operator"
   ```

   > **NOTE:** The `"operator.kyma-project.io/managed-by": "lifecycle-manager"` label is mandatory for the Lifecycle Manager runtime controller to know which resources to cache.

3. Deploy the secret in the same cluster where the ModuleTemplate is to be located i.e if the ModuleTemplate is in the SKR cluster, then the secret should be deployed to the SKR, otherwise it should be deployed to the KCP cluster.

### Generate a ModuleTemplate CR with the `oci-registry-cred` label

The `oci-registry-cred` label in a ModuleTemplate CR allows Lifecycle Manager to parse the Secret label selector and propagate it to the Manifest CR so that Lifecycle Manager knows which credentials Secret to look up.

To support the ModuleTemplate CR with the `oci-registry-cred` label, use Kyma CLI with the `registry-cred-selector` flag for creating a module command.

For example, you can run the following command to push your module image and generate a ModuleTemplate CR with the `oci-registry-cred` label:

   ```sh
   kyma alpha create module -n [name]  --version [module version] --registry [private oci registry] -w -c [access credential with write permission] --registry-cred-selector=operator.kyma-project.io/oci-registry-cred=test-operator
   ```

Verify in each **descriptor.component.resources** layer, if it contains the `oci-registry-cred` label.

   ```yaml
   descriptor:
    component:
      resources:
      - access:
          digest: sha256:bc2a16d9b01b4809f8123e9402e2c6e6be2ef815975ad4131282ceb33af4d5a5
          type: localOciBlob
        labels:
        - name: oci-registry-cred
          value:
            operator.kyma-project.io/oci-registry-cred: test-operator
   ```

With this ModuleTemplate CR, Lifecycle Manager should access a private OCI registry without any authentication problems.
