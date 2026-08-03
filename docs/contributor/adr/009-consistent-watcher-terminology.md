# ADR 009 - Consistent Watcher Terminology

## Status

Proposed

## Context

The naming applied to the Watcher mechanism and its Public Key Infrastructure (PKI) is inconsistent across code, configuration, and documentation. The same concept carries a different name in each layer, and some names actively mislead.

The following examples illustrate the problem:

- The mechanism is called `runtime watcher` in official material, `Watcher` in the custom resource and controller, and `SkrWebhook` in the deployed artifact.
- The certificate hierarchy is described as server certificate and client certificate in [ADR 007](007-pki-certs-and-rotation.md), as `root`, `serving`, and `selfsigned` in configuration, and as `SkrCertificate` in code.
- The Issuer named `klm-watcher-selfsigned` is a Certificate Authority (CA) Issuer that signs client certificates. It is not self-signed. The name contradicts its function.
- The command-line flags use the `--self-signed-cert-*` prefix for certificates that follow the server and client roles defined in ADR 007.

This ADR aligns the terminology and records the target names. It extends the naming guidelines of [ADR 005](005-consistent-naming.md) and reuses the certificate vocabulary of [ADR 007](007-pki-certs-and-rotation.md).

This ADR does not, on its own, rename any code or configuration. It records the agreed vocabulary and the target names so that follow-up changes can apply them incrementally.

## Decision

### Canonical Terms

The following canonical terms apply to the mechanism and its parts:

| Concept | Canonical Term | Definition |
|---------|----------------|------------|
| The mechanism, on the Kyma Control Plane (KCP) side | Watcher | The KCP-side mechanism that receives change notifications from runtimes. Covers the Watcher custom resource, the Watcher controller, and the Istio routing KLM configures for it. |
| The deployed agent, on the runtime side | Runtime Watcher | The separate component from the `runtime-watcher` repository that KLM deploys into the runtime. The term refers only to that component and its image. |
| The self-signed CA certificate | CA certificate | The self-signed certificate that anchors the Watcher PKI. It also serves as the server certificate. See ADR 007. |
| The certificate the gateway presents | Server certificate | The certificate the `klm-watcher` Istio gateway presents to the runtime. Equal to the CA certificate. See ADR 007. |
| The per-runtime certificate | Client certificate | The certificate the Runtime Watcher presents to the gateway. Signed by the CA certificate. One per runtime. See ADR 007. |

The term Watcher refers to KLM's side of the mechanism. The term Runtime Watcher refers to the component deployed into the runtime. Keeping the two distinct removes the central ambiguity, because `runtime watcher` previously named both the end-to-end mechanism and the deployed component.

The certificate vocabulary follows ADR 007. The mechanism uses a self-signed CA certificate that doubles as the server certificate, and per-runtime client certificates signed by that CA. The terms root, leaf, and self-signed are not used to describe these certificates, because they either lose the server-and-client role distinction or contradict the actual function.

### Naming Scheme by Layer

The scheme separates two categories:

- **Frozen names** are operational contracts. Kubernetes resource names, secret names, and command-line flags are consumed by live landscapes and external tooling. Renaming them is a breaking change that requires a migration. This ADR keeps them as they are and documents their meaning.
- **Target names** apply to Go identifiers and documentation. They are free to change and represent the agreed end state. This ADR records them; it does not apply them.

#### Configuration and Resources (Frozen)

The following Kubernetes resource and secret names remain unchanged. The table records what each one is, so the frozen name is at least understood:

| Current Name | Kind | Role in the PKI |
|--------------|------|-----------------|
| `klm-watcher-root` | Issuer | Self-signed bootstrap Issuer that issues the CA certificate. |
| `klm-watcher-serving` | Certificate | The self-signed CA certificate. Also the server certificate. |
| `klm-watcher-selfsigned` | Issuer | The CA Issuer that signs client certificates. Despite the name, it is not self-signed. |
| `klm-watcher` | Secret | Stores the CA certificate, in the `istio-system` namespace. Referred to as the root secret in code. |
| `klm-istio-gateway` | Secret | Stores the server certificate and the CA bundle. Referred to as the gateway secret in code. |
| `{KYMA_NAME}-webhook-tls` | Secret | Stores a client certificate on KCP, one per runtime. |
| `skr-webhook-tls` | Secret | The client certificate and CA bundle synced to the runtime. |
| `skr-webhook` | Deployment | The Runtime Watcher deployment in the runtime. |

#### Command-Line Flags (Frozen)

The `--self-signed-cert-*` flags keep their names to avoid breaking landscape configuration. Their documentation must state that the flags configure the CA certificate and its Issuer, following the server and client roles of ADR 007.

#### Go Code (Target)

The following target names apply to Go identifiers. This ADR records them as the agreed end state; a follow-up applies them:

| Concept | Current | Target |
|---------|---------|--------|
| The manifest manager type | `SkrWebhookManifestManager` | `WatcherResourceReconciler` |
| The certificate service interface | `SKRCertificateService` | `ClientCertificateService` |
| Create the client certificate | `CreateSkrCertificate` | `CreateClientCertificate` |
| Renew the client certificate | `RenewSkrCertificate` | `RenewClientCertificate` |
| Delete the client certificate | `DeleteSkrCertificate` | `DeleteClientCertificate` |
| Check renewal is overdue | `IsSkrCertificateRenewalOverdue` | `IsClientCertificateRenewalOverdue` |
| Get the client certificate secret | `GetSkrCertificateSecret` | `GetClientCertificateSecret` |
| The client certificate name helper | `name.SkrCertificate` | `name.ClientCertificate` |
| The CA bundle timestamp accessor | `getGatewaySecretCaBundleExtendedAtTime` | `getCaAddedToBundleAtTime` |
| The self-signed cert flag fields | `SelfSignedCert*` | `CaCertificate*` |

The `Manager` suffix is dropped from the manifest manager type, because it adds no information beyond the layer suffix already defined by ADR 005.

The variable spellings for the `caAddedToBundleAt` annotation converge on one form. The annotation is `caAddedToBundleAt`. Code currently reads it through variables named `caBundleExtendedAt`, `caBundledAt`, and `getGatewaySecretCaBundleExtendedAtTime`. The target uses `caAddedToBundleAt` consistently.

#### Documentation (Target)

Documentation uses the canonical terms. Existing documents are updated as they are touched. Component tables that list frozen resource names keep those names and add the canonical term in the description.

## Consequences

- The project has one agreed vocabulary for the Watcher mechanism and its PKI, aligned with ADR 007.
- Frozen names stay stable, so no landscape migration is required for this ADR.
- Code and documentation diverge from the canonical terms until follow-up changes apply the target names. The divergence is bounded, because the target names are recorded here.
- The misleading `klm-watcher-selfsigned` Issuer name persists as a frozen contract. Its documentation must state that it is the CA Issuer, so readers are not misled.
- This ADR is the reference for the implementation detail requested in the follow-up issue.
