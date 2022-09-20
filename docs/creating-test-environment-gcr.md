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

2. Export environment variables.

   ```sh
   export MODULE_REGISTRY=europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   export IMG_REGISTRY=$MODULE_REGISTRY/operator-images
   ```

   > **NOTE:** For `MODULE_REGISTRY`, do not define any scheme such as `https://`, otherwise the module isn't generated properly. The scheme is appended automatically in the operators based on the environment.

3. To make it work with remote clusters such as in Gardener, specify that Read access to the repository is possible anonymously:

   ```sh
   gcloud artifacts repositories add-iam-policy-binding operator-test \
    --location=europe-west3 --member=allUsers --role=roles/artifactregistry.reader

### Authenticate locally

We will assume you will be [creating and using a service-account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) called `operator-test-sa`.

4. Authenticate against your registry:

   ```sh
   gcloud auth configure-docker \
       europe-west3-docker.pkg.dev

### Create a service account in Google Cloud.

5. For productive purposes, create a service account. In this example, call it `operator-test-sa`.

   ```sh
   gcloud iam service-accounts create operator-test-sa \
       --display-name="Operator Test Service Account"

6. To get the necessary permissions, assign roles to your service account.

   > **TIP:** For details, read [Required roles](https://cloud.google.com/iam/docs/creating-managing-service-accounts#permissions).

   ```sh
   gcloud projects add-iam-policy-binding sap-kyma-jellyfish-dev \
         --member='serviceAccount:operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com' \
         --role='roles/artifactregistry.reader' \
         --role='roles/artifactregistry.writer'

7. Impersonate the service account:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com

8. Verify your login:

   ```sh
   gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com | docker login -u oauth2accesstoken --password-stdin https://europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/operator-test
   ```

   export `GCR_DOCKER_PASSWORD` for `operator/docker-push` and `MODULE_CREDENTIALS` for `module-build` make command:

   ```sh
   export MODULE_CREDENTIALS=oauth2accesstoken:$(gcloud auth print-access-token --impersonate-service-account operator-test-sa@sap-kyma-jellyfish-dev.iam.gserviceaccount.com)
