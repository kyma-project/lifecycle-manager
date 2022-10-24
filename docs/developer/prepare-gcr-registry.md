# Create a test environment on Google Container Registry (GCR)

## Prerequisites

You are using GCP Artifact Registry. The following instructions assume that you have a GCP project called `sap-kyma-jellyfish-dev`.

## Instructions

### Create your repository

1. Create an Artifact Registry repository. For this example, call it `operator-test`:

   ```sh
   gcloud artifacts repositories create operator-test \
       --repository-format=docker \
       --location europe-west3

2. Use the created registries.

   ```sh
   export MODULE_REGISTRY=europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   export IMG_REGISTRY=$MODULE_REGISTRY/operator-images
   ```

   > **NOTE:** For `MODULE_REGISTRY`, do not define any scheme such as `https://`, otherwise the module isn't generated properly. The scheme is appended automatically in the operators based on the environment.

3. To make it work with remote clusters such as in Gardener, specify that Read access to the repository, if possible anonymously:

   ```sh
   gcloud artifacts repositories add-iam-policy-binding operator-test \
    --location=europe-west3 --member=allUsers --role=roles/artifactregistry.reader

### Authenticate locally

Under the assumption you're [creating and using a service-account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) called `operator-test-sa`.

1. Authenticate against your registry:

   ```sh
   gcloud auth configure-docker \
       europe-west3-docker.pkg.dev

### Create a service account in Google Cloud

1. For productive purposes, create a service account. In this example, call it `operator-test-sa`.

   ```sh
   gcloud iam service-accounts create operator-test-sa \
       --display-name="Operator Test Service Account"

2. To get the necessary permissions, assign roles to your service account.

   > **TIP:** For details, read [Required roles](https://cloud.google.com/iam/docs/creating-managing-service-accounts#permissions).

   ```sh
   gcloud projects add-iam-policy-binding sap-kyma-jellyfish-dev \
         --member='serviceAccount:operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com' \
         --role='roles/artifactregistry.reader' \
         --role='roles/artifactregistry.writer'

3. Impersonate the service account:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com

4. Adjust the `docker-push` command in `Makefile`
   ```makefile
   .PHONY: docker-push
   docker-push: ## Push docker image with the manager.
   ifneq (,$(GCR_DOCKER_PASSWORD))
     docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
   endif
   docker push ${IMG}
   ```

5. Verify your login:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com | docker login -u oauth2accesstoken --password-stdin https://europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   ```
   export `GCR_DOCKER_PASSWORD` for `docker-push` make command:
   ```sh
   export GCR_DOCKER_PASSWORD=$(gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com)
   ```
   
   Use the following setup in conjunction with the kyma CLI:
   ```sh
   kyma alpha create module kyma-project.io/module/template 0.0.1 . -w -c oauth2accesstoken:$GCR_DOCKER_PASSWORD
   ```