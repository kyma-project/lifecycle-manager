# ADR 007 - PKI Certificates and Zero-Downtime Rotation

## Status

Accepted

## Context

Lifecycle Manager (KLM) uses a Public Key Infrastructure (PKI) to secure communication between the Runtime Watcher on runtimes and KLM on Kyma Control Plane (KCP).
This ADR serves as documentation what CA, intermediate and client certificates exist and how they are rotated.

## Decision

> In the following, the term *"server certificate"* is used for the certificate the *klm-watcher* gateway presents to the *skr-webhook* deplyoment.
> The term *"client certificate"* is used for the certificate the *skr-webhook* deployment presents to the klm-watcher gateway.

It is decided that only a CA certificate and client certificates signed by this CA are used. Intermediate certificates are skipped for simplicity as no usage outside of Runtime Watcher mechanism is expected and since there is no need for individually rotating CA, server and client certificates. The CA certificate therefore also serves as the server certificate.

The resulting setup is depicted in the figure below. Only the secrets storing the certificates are shown, the related *Certificate* Custom Resources are skipped for simplicity. The color orange indicates the current CA certificate while yellow indicates the previous CA certificate before rotation. Purple indicates a client certificate. The CA certificate is stored in the secret *klm-watcher*. This CA certificate is reused as the server certificate in the secret *klm-istio-gateway*. Additionally, all previously existing CA certificates are stored in the CA bundle in the secret *klm-istio-gateway* until they have expired. The client certificates are stored in *\*-webhook-tls* secrets and synced to the SKR together with the CA bundle from the *klm-istio-gateawy* secret as secret named *skr-webhook-tls*.

> In restricted markets, *Gardener certificate-management* is used instead of *cert-manager*. For simplicity, this is ignored in the figure and only *cert-manager* is shown.

Upon rotation of the CA certificate, the following steps are executed.

### 1 - Rotate CA certificate

Once due, *cert-manager* issues a new self-signed CA certificate and stores it in the *klm-watcher* secret (1). This is done automatically by *cert-manager* based on certificate duration and renew before buffer.

### 2 - Update server CA bundle

KLM watches the *klm-watcher* certificate (2a). When it changes, KLM adds the new CA certificate to the CA bundle in *klm-istio-gateway* (2b). It further adds the `lastModifiedAt` annotation to the *klm-istio-gateway* secret indicating the last time the CA bundle was updated. In this step, also all previously stored CA certificates stored in the CA bundle are removed if they have expired.

### 3 - Delete client certificate secrets

As part of the Kyma CR reconciliation, KLM gets the *klm-istio-gateway* to determine the last time the CA bundle was updated indicated by the `lastModifiedAt` annotation (3a). If the CA bundle is newer than the *\*-webhook-tls* secret, KLM deletes this secret (3b).

> In restricted markets where *Gardener certificate-management* is used instead of *cert-manager*, the secret is not deleted. Instead, the `renew: true` spec field is set on the related certificate resource.

### 4 - Re-issue client certificates

The deleted *\*-webhook-tls* secret forces *cert-manager* to re-issue the client certificate (4). Since the CA certificate has been rotated, the new client certificate is signed by the new CA certificate.

### 5 - Sync re-issued client certificates to SKR

As part of the next Kyma CR reconciliation, KLM again gets the *klm-istio-gateway* secret to determine the last time the CA bundle was updated (5a). Now, since the *\*-webhook-tls* secret containing the related client certificate is newer than the `lastModifiedAt` annotation of the *klm-istio-gateway* (5b), KLM doesn't delete the *\*-webhook-tls* secret again. Instead, it syncs the client certificate together with the updated CA bundle to the SKR (5c).

### 6 - Switch the server certificate

After some grace period, KLM switches the server certificate stored in *klm-istio-gateway* for the latest CA certificate in *klm-watcher* (6). This is done as part of the regular reconciliation of the *klm-watcher* secret.

![](../assets/watcher-certificates.drawio.svg)

## Consequences

The following key criteria **MUST** be ensured:

- **3b** MUST only happen once the new CA certificate has been added to the CA bundle in *klm-istio-gateway* (2b)
  - otherwise, client certificates may be generated that the server doesn't trust yet
- **6** MUST only happen after all client certificates have been renewed with the new CA certificate (4) and synced to the SKR (5c)
  - otherwise, clients that haven't received the new CA bundle don't trust the server anymore

As per the setup described above and the key criteria, the following holds true and therefore we have a zero-downtime rotation:

- the *klm-watcher* gateway trusts all clients certificates until their expiry since they have been signed by a certificate stored in the *klm-istio-gateway* CA bundle
- the *skr-webhook* deployment trust the server certificate until its expiry as it is the same as one CA certificate stored in the *skr-webhook-tls* CA bundle
