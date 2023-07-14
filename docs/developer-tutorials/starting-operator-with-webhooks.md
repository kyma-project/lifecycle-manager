# Enable webhooks in Lifecycle Manager

## Context

To make local testing easier, webhooks are disabled by default. To enable webhooks running with the operator, you must change some `kustomization.yaml` files as well as introduce a flag that will enable the webhook server.

For further information, read the [kubebuilder tutorial](https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html).

## Procedure

1. Go to [`config/crd/kustomization.yaml`](https://github.com/kyma-project/lifecycle-manager/blob/main/config/crd/kustomization.yaml). Follow the instructions from the file to uncomment sections referring to [WEBHOOK] and [CERT_MANAGER].

2. Go to [`config/default/kustomization.yaml`](https://github.com/kyma-project/lifecycle-manager/blob/main/config/default/kustomization.yaml). Follow the instruction in the file and uncomment all sections referring to [WEBHOOK], [CERT-MANAGER], and [PROMETHEUS].

3. Enable the webhooks by setting the `--enable-webhooks` flag. Run:

   ```bash
   go run ./main.go ./flags.go --enable-webhooks
   ```

   You can also enable webhooks by updating `config/manager/controller_manager_config.yaml`:

   ```yaml
   apiVersion: controller-runtime.sigs.k8s.io/v1alpha1
   kind: ControllerManagerConfig
   health:
     healthProbeBindAddress: :8081
   metrics:
     bindAddress: 127.0.0.1:8080
   webhook:
     port: 9443
   leaderElection:
     leaderElect: true
     resourceName: 893110f7.kyma-project.io
   enableWebhooks: true
   ```
