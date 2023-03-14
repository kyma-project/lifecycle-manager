# Provide credential for private OCI registry authentication

## Prerequisites

You are using private OCI registry for hosting module resources. The following instructions guide you to provide the credential for authentication.

## Instructions

### Get registry credentials

For private registry which requires access permissions, normally each major registry provider offers standard docker access credential with username and password as combination.

As an example, Docker Hub lets you create personal [access tokens](https://docs.docker.com/docker-hub/access-tokens/) as alternatives to your password. 

Prepare your credential first before next steps. Be aware, how to deal with credential rotation is not covered in this guide.

_Notice, to compliance least privileges principle, make sure this credential only have read only permission._

### Prepare a docker-registry secret manifest

1. Create a docker-registry secret manifest
   ```sh
   kubectl create secret docker-registry [secret name] --docker-server=[your oci registry host] --docker-username=[username] --docker-password=[password/token]  --dry-run=client -oyaml > registry_cred_secret.yaml

2. Add some labels to the secret so that it can be configured later in the module template as label selector.
   ```yaml
   apiVersion: v1
   kind: Secret
   labels:
   "operator.kyma-project.io/oci-registry-cred": "test-operator"
   
3. Deploy to the KCP cluster in each environment.

### Generate Module Template with `oci-registry-cred` label

`oci-registry-cred` label in module template allows lifecycle manager parsing the secret label selector and propagate to manifest CR so that module manager knows which credential secret to lookup.

The most convenient way to support module template with `oci-registry-cred` label is using Kyma CLI with `registry-cred-selector`flag for create module command.

For example, you can run following command to push your module image and generate the module template with `oci-registry-cred` label
   ```sh
   kyma alpha create module -n [name]  --version [module version] --registry [private oci registry] -w -c [access credental with write permission] --registry-cred-selector=operator.kyma-project.io/oci-registry-cred=test-operator
   ```
Verify in each component descriptor resources layer, it should contains `oci-registry-cred` label.
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
With this module template, module manager should access private oci registry with no authentication problem.