# ADR 009 - Consistent Watcher Terminology

## Status

Accepted

## Context

The naming applied to the Watcher mechanism and its Public Key Infrastructure (PKI) is inconsistent across code, configuration, and documentation. The same concept often carries a different name in each layer, and some names contradict what they describe.

The following examples illustrate the problem:

- The mechanism is called `runtime watcher` in official material, `Watcher` in the custom resource and controller, and `SkrWebhook` in the deployed artifact.
- The certificate hierarchy is described as server certificate and client certificate in [ADR 007](007-pki-certs-and-rotation.md), as `root`, `serving`, and `selfsigned` in configuration, and as `SkrCertificate` in code.
- The Issuer named `klm-watcher-selfsigned` is a Certificate Authority (CA) Issuer that signs client certificates. It is not self-signed, so the name contradicts its function.
- The command-line flags use the `--self-signed-cert-*` prefix for certificates that follow the server and client roles defined in ADR 007.

A reader who moves between the code, the manifests, and the documentation must repeatedly re-map one concept onto several names. This slows down onboarding, makes incident response harder, and hides the actual structure of the PKI.

This ADR defines one vocabulary for the mechanism and its PKI, and records the target name for every part in every layer. It extends the naming guidelines of [ADR 005](005-consistent-naming.md) and reuses the certificate vocabulary of [ADR 007](007-pki-certs-and-rotation.md).

## Decision

Names describe function in the solution domain. A name states what a thing is and what role it plays, not where it happens to sit or how it was bootstrapped. The same concept uses the same name in code, configuration, and documentation.

### Canonical Terms

The following canonical terms apply to the mechanism and its parts:

| Concept | Canonical Term | Definition |
|---------|----------------|------------|
| The mechanism, on the Kyma Control Plane (KCP) side | Watcher | The KCP-side mechanism that receives change notifications from runtimes. Covers the Watcher custom resource, the Watcher controller, and the Istio routing KLM configures for it. |
| The deployed agent, on the runtime side | Runtime Watcher | The separate component from the `runtime-watcher` repository that KLM deploys into the runtime. The term refers only to that component and its image. |
| The self-signed CA certificate | CA certificate | The self-signed certificate that anchors the Watcher PKI. It also serves as the server certificate. See ADR 007. |
| The certificate the gateway presents | Server certificate | The certificate the Watcher Istio gateway presents to the runtime. Equal to the CA certificate. See ADR 007. |
| The per-runtime certificate | Client certificate | The certificate the Runtime Watcher presents to the gateway. Signed by the CA certificate. One per runtime. See ADR 007. |

Two rules resolve the central ambiguities:

- **Watcher and Runtime Watcher are distinct.** Watcher refers to KLM's side of the mechanism. Runtime Watcher refers to the component deployed into the runtime. Previously, `runtime watcher` named both, which is the main source of confusion.
- **The PKI uses the ADR 007 vocabulary.** The mechanism uses a self-signed CA certificate that doubles as the server certificate, and per-runtime client certificates signed by that CA. The terms root, leaf, and self-signed are not used for these certificates, because they either lose the server-and-client role distinction or contradict the actual function.

### Naming Scheme

The scheme renames every part that does not already follow the canonical terms. It applies to Kubernetes resources, secrets, command-line flags, Go identifiers, and documentation, so that one concept reads the same everywhere.

#### Kubernetes Resources and Secrets

| Current Name | Kind        | Target Name                  | Rationale                                                                  |
|--------------|-------------|------------------------------|----------------------------------------------------------------------------|
| `klm-watcher-root` | Issuer      | `klm-watcher-ca`             | It is the self-signed Issuer that bootstraps the CA certificate.           |
| `klm-watcher-serving` | Certificate | `klm-watcher-ca`             | It is the CA certificate, which also serves as the server certificate.     |
| `klm-watcher` | Secret      | `klm-watcher-ca`             | Stores the CA certificate. Matches the Certificate name.                   |
| `klm-istio-gateway` | Secret      | `klm-watcher-server`         | Stores the server certificate and the CA bundle used by the gateway.       |
| `klm-watcher-selfsigned` | Issuer      | `klm-watcher-client`         | It is the CA Issuer that signs client certificates. It is not self-signed. |
| `{KYMA_NAME}-webhook-tls` | Certificate | `{KYMA_NAME}-watcher-client` | Requests a client certificate on KCP, one per runtime. Matches the Secret name.       |
| `{KYMA_NAME}-webhook-tls` | Secret      | `{KYMA_NAME}-watcher-client` | Stores a client certificate on KCP, one per runtime.                       |
| `skr-webhook-tls` | Secret      | `runtime-watcher-client`     | The client certificate and CA bundle synced to the runtime.                |
| `skr-webhook` | Deployment  | `runtime-watcher`            | The Runtime Watcher deployment in the runtime.                             |

#### Command-Line Flags

The `--self-signed-cert-*` flags are renamed to `--watcher-client-cert-*`, because they configure the per-runtime client certificate, not the CA certificate. The `--self-signed-cert-issuer-*` flags name the Issuer that signs those client certificates, so they follow the same `client-cert` prefix.

| Current Flag | Target Flag |
|--------------|-------------|
| `--self-signed-cert-duration` | `--watcher-client-cert-duration` |
| `--self-signed-cert-renew-before` | `--watcher-client-cert-renew-before` |
| `--self-signed-cert-renew-buffer` | `--watcher-client-cert-renew-buffer` |
| `--self-signed-cert-key-size` | `--watcher-client-cert-key-size` |
| `--self-signed-cert-issuer-name` | `--watcher-client-cert-issuer-name` |
| `--self-signed-cert-issuer-namespace` | `--watcher-client-cert-issuer-namespace` |

The duration, renewal, and key size of the CA certificate are not configured by any flag. They are set on the CA Certificate resource in `config/certmanager/certificate_watcher.yaml`. Naming these flags after the CA would imply control the flags do not have.

#### Go Identifiers

| Concept | Current | Target |
|---------|---------|--------|
| The watcher resource service type | `SkrWebhookManifestManager` | `watcher.Service` |
| The certificate service interface | `SKRCertificateService` | `ClientCertificateService` |
| Create the client certificate | `CreateSkrCertificate` | `CreateClientCertificate` |
| Renew the client certificate | `RenewSkrCertificate` | `RenewClientCertificate` |
| Delete the client certificate | `DeleteSkrCertificate` | `DeleteClientCertificate` |
| Check renewal is overdue | `IsSkrCertificateRenewalOverdue` | `IsClientCertificateRenewalOverdue` |
| Get the client certificate secret | `GetSkrCertificateSecret` | `GetClientCertificateSecret` |
| The client certificate name helper | `name.SkrCertificate` | `name.ClientCertificate` |
| The CA bundle timestamp accessor | `getGatewaySecretCaBundleExtendedAtTime` | `getCaAddedToBundleAtTime` |
| The client cert flag fields | `SelfSignedCert*` | `WatcherClientCert*` |

The `SkrWebhookManifestManager` type installs and removes the watcher resources on the runtime and orchestrates the client certificate lifecycle. It lives in package `watcher` and is a service in the sense of ADR 005, so it becomes `watcher.Service`. The `Manager` suffix and the `Watcher` prefix are both dropped: ADR 005 sanctions the `Service` suffix and establishes context from the package name, so a `Watcher` prefix would only stutter as `watcher.Watcher...`. The `Reconciler` suffix is reserved for controller-runtime reconcilers, which this type is not.

The variable spellings for the `caAddedToBundleAt` annotation converge on one form. The annotation is `caAddedToBundleAt`. Code currently reads it through variables named `caBundleExtendedAt`, `caBundledAt`, and `getGatewaySecretCaBundleExtendedAtTime`. The target uses `caAddedToBundleAt` consistently.

#### Documentation

Documentation uses the canonical terms. Kubernetes resource names in component tables and reference material are updated to the target names. Existing documents are updated as they are touched.

### Migration

Kubernetes resource names, secret names, and command-line flags are operational contracts. Live landscapes reference them, and the Watcher PKI performs zero-downtime certificate rotation (see ADR 007), so a name cannot simply change in place without breaking the mTLS trust chain.

Renames of these contracts follow a transition that keeps the old and new names valid at the same time:

1. Introduce the new name alongside the old one.
2. Migrate producers and consumers to the new name.
3. Remove the old name once no landscape references it.

Command-line flags keep their old form as a deprecated alias for one release before removal. Go identifiers and documentation carry no external contract and are renamed directly.

## Consequences

- The project has one agreed vocabulary for the Watcher mechanism and its PKI, aligned with ADR 007. One concept reads the same in code, configuration, and documentation.
- Names describe function, so the misleading `selfsigned` Issuer name and the ambiguous `runtime watcher` term are resolved rather than documented around.
- Renaming operational contracts requires a staged migration per landscape. The effort is one-time and is scoped as follow-up work.
- Until the migration completes, some names still use the old form. The target names recorded here bound that transitional state and serve as the reference for the follow-up implementation.
