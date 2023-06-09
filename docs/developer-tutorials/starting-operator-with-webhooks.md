# Enabling webhooks in Lifecycle Manager

## Context

To make local testing easier, webhooks are disabled by default. To enable webhooks running with the operator, you must change some kustomization.yaml files as well as introduce a flag that will enable the webhook server.

For further information, read the [kubebuilder tutorial](https://book.kubebuilder.io/cronjob-tutorial/running-webhook.html)

## Procedure

1. In `config/crd/kustomization.yaml`:

   ```yaml
   patchesStrategicMerge:
     # [WEBHOOK] To enable webhook, uncomment all the sections with [WEBHOOK] prefix.
     # patches here are for enabling the conversion webhook for each CRD
     - patches/webhook_in_kymas.yaml
     - patches/webhook_in_moduletemplates.yaml
     #+kubebuilder:scaffold:crdkustomizewebhookpatch

     # [CERTMANAGER] To enable cert-manager, uncomment all the sections with [CERTMANAGER] prefix.
     # patches here are for enabling the CA injection for each CRD
     - patches/cainjection_in_kymas.yaml
     - patches/cainjection_in_moduletemplates.yaml
   #+kubebuilder:scaffold:crdkustomizecainjectionpatch
   ```

2. In `config/default/kustomization.yaml`:

   ```yaml
   bases:
   ---
   - ../crd
   - ../rbac
   - ../manager
   # [WEBHOOK] To enable webhook, uncomment all the sections with [WEBHOOK] prefix including the one in
   # crd/kustomization.yaml
   #- ../webhook
   # [CERTMANAGER] To enable cert-manager, uncomment all sections with 'CERTMANAGER'. 'WEBHOOK' components are required.
   #- ../certmanager
   # [PROMETHEUS] To enable prometheus monitor, uncomment all sections with 'PROMETHEUS'.
   - ../prometheus
   ```

3. Enable the webhooks by configuring the `enable-webhooks` flag:

   ```go
   flag.BoolVar(&flagVar.enableWebhooks, "enable-webhooks", false,
       "Enabling Validation/Conversion Webhooks.")
   ```

   You can do this by updating `config/manager/controller_manager_config.yaml`:

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
