1. AOI that should be covered:
- Production Security Hardening: Implement mTLS or API token authentication for HTTP endpoints instead of relying solely on Tailnet perimeter trust.
- Observability Stack: Integrate structured logging, Prometheus metrics, and OpenTelemetry tracing to replace raw string logging.
- Idempotency & Failure Recovery: Ensure multi-step control plane operations are fully idempotent and handle partial-failure rollbacks gracefully.

2. OCI Pillar
- The system should come with the tooling and piping to manage OCI intialization of nodes, enabling users to handle OCI vms's in similar fashion to what the vms script does.