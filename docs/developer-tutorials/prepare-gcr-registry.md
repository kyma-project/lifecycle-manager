# Create a test environment on Google Container Registry (GCR)

## Context

If you use the GCP Artifact Registry, follow these instructions to create a test environment.

## Prerequisites

This tutorial assumes that you have a GCP project called `sap-kyma-jellyfish-dev`.

## Procedure

### Create your repository

1. Create an Artifact Registry repository. For tutorial purposes, call it `operator-test`.

   ```sh
   gcloud artifacts repositories create operator-test \
       --repository-format=docker \
       --location europe-west3
   ```

2. To make it work with remote clusters such as in Gardener, specify the Read access to the repository, if possible anonymously:

   ```sh
   gcloud artifacts repositories add-iam-policy-binding operator-test \
    --location=europe-west3 --member=allUsers --role=roles/artifactregistry.reader
   ```

### Authenticate locally and create a service account in Google Cloud

1. Under the assumption you're [creating and using a service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) called `operator-test-sa`, authenticate against your registry:

   ```sh
   gcloud auth configure-docker \
       europe-west3-docker.pkg.dev
   ```

2. For productive purposes, create a service account. For tutorial purposes, call it `operator-test-sa`.

   ```sh
   gcloud iam service-accounts create operator-test-sa \
       --display-name="Operator Test Service Account"

3. To get the necessary permissions, assign roles to your service account.

   > **TIP:** For details, read [Required roles](https://cloud.google.com/iam/docs/creating-managing-service-accounts#permissions).

   ```sh
   gcloud projects add-iam-policy-binding sap-kyma-jellyfish-dev \
         --member='serviceAccount:operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com' \
         --role='roles/artifactregistry.reader' \
         --role='roles/artifactregistry.writer'
   ```

4. Impersonate the service account:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com
   ```

5. Verify your login:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com | docker login -u oauth2accesstoken --password-stdin https://europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   ```

   Export `GCR_DOCKER_PASSWORD` for the `docker-push` make command:

   ```sh
   export GCR_DOCKER_PASSWORD=$(gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com)
   ```

6. Adjust the `docker-push` command in `Makefile`:

   ```makefile
   .PHONY: docker-push
   docker-push: ## Push docker image with the manager.
   ifneq (,$(GCR_DOCKER_PASSWORD))
     docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
   endif
   docker push ${IMG}
   ```

7. Use the following setup in conjunction with Kyma CLI:

   ```sh
   kyma alpha create module kyma-project.io/module/template 0.0.1 . -w -c oauth2accesstoken:$GCR_DOCKER_PASSWORD
   ```
