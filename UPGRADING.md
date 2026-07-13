# Upgrading to 1.3.0

- The chart no longer creates a ClusterRole or ClusterRoleBinding because the
  scaler does not access the Kubernetes API.
- `serviceAccount.automountServiceAccountToken` now defaults to `false`.
- Use `secret.existingSecret` for an existing Secret. The old top-level
  `secretName` remains as a deprecated fallback only with `secret.create=false`.
- A chart-created Secret requires non-empty `secret.data`. Its name and key are
  configured with `secret.name` and `secret.key`; its default name now follows
  the Helm release fullname.
- Resource names now follow Helm conventions. The recommended release name
  `yc-keda-external-scaler` preserves the existing service name and address.
- `image.digest` takes precedence over `image.tag`; an empty tag uses
  `Chart.appVersion`.
- An explicitly invalid `targetValue` is now reported as a scaler configuration
  error instead of silently falling back to `80`.
