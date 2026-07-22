#!/usr/bin/env bash
set -euo pipefail

chart="${CHART_DIR:-helm/yc-keda-external-scaler}"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

helm template yc-keda-external-scaler "$chart" \
  --set-string secret.data=test-key >"$workdir/default.yaml"
grep -q '^kind: Secret$' "$workdir/default.yaml"
grep -q 'automountServiceAccountToken: false' "$workdir/default.yaml"
grep -q 'image: "cr.yandex/sol/keda/yc-keda-external-scaler:v1.4.0"' "$workdir/default.yaml"
grep -q 'value: "authorizedKey"' "$workdir/default.yaml"
grep -q 'name: yc-keda-external-scaler' "$workdir/default.yaml"
grep -q 'secretName: yc-keda-external-scaler' "$workdir/default.yaml"
if grep -Eq '^kind: ClusterRole(Binding)?$' "$workdir/default.yaml"; then
  echo "unexpected cluster RBAC resource" >&2
  exit 1
fi

helm template scaler "$chart" \
  --set secret.existingSecret=precreated-key \
  --set secret.key=credentials.json >"$workdir/existing.yaml"
if grep -q '^kind: Secret$' "$workdir/existing.yaml"; then
  echo "existingSecret mode rendered a Secret" >&2
  exit 1
fi
grep -q 'secretName: precreated-key' "$workdir/existing.yaml"
grep -q 'subPath: "credentials.json"' "$workdir/existing.yaml"

helm template scaler "$chart" \
  --set secret.create=false \
  --set secretName=legacy-key >"$workdir/legacy.yaml"
grep -q 'secretName: legacy-key' "$workdir/legacy.yaml"

helm template scaler "$chart" \
  --set-string secret.data=test-key \
  --set fullnameOverride=custom-scaler \
  --set image.tag=v9.9.9 >"$workdir/overrides.yaml"
grep -q 'name: custom-scaler' "$workdir/overrides.yaml"
grep -q 'image: "cr.yandex/sol/keda/yc-keda-external-scaler:v9.9.9"' "$workdir/overrides.yaml"

helm template named "$chart" \
  --set-string secret.data=test-key \
  --set nameOverride=custom-name >"$workdir/name-override.yaml"
grep -q 'app.kubernetes.io/name: custom-name' "$workdir/name-override.yaml"
grep -q -- "- 'custom-name'" "$workdir/name-override.yaml"

digest="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
helm template scaler "$chart" \
  --set-string secret.data=test-key \
  --set image.tag=v9.9.9 \
  --set image.digest="$digest" >"$workdir/digest.yaml"
grep -q "image: \"cr.yandex/sol/keda/yc-keda-external-scaler@$digest\"" "$workdir/digest.yaml"

if helm template scaler "$chart" >"$workdir/missing-data.yaml" 2>"$workdir/missing-data.err"; then
  echo "chart rendered without required secret.data" >&2
  exit 1
fi

if helm template scaler "$chart" \
  --set secret.create=false >"$workdir/missing-existing.yaml" 2>"$workdir/missing-existing.err"; then
  echo "secret.create=false rendered without an existing Secret name" >&2
  exit 1
fi

helm template scaler "$chart" \
  --set auth.workloadIdentityFederation.serviceAccountID=wlif-service-account \
  --set auth.workloadIdentityFederation.audience=https://storage.example.test/mk8s-oidc/cluster \
  --set-string secret.data=ignored-key >"$workdir/wlif.yaml"
grep -q 'yandex.cloud/federated-yc-service-account-id: "wlif-service-account"' "$workdir/wlif.yaml"
grep -q 'value: "workloadIdentityFederation"' "$workdir/wlif.yaml"
grep -q 'value: "https://auth.yandex.cloud/oauth/token"' "$workdir/wlif.yaml"
grep -q 'value: "/var/run/secrets/tokens/yc-wlif-token"' "$workdir/wlif.yaml"
grep -q 'audience: "https://storage.example.test/mk8s-oidc/cluster"' "$workdir/wlif.yaml"
grep -q 'expirationSeconds: 3600' "$workdir/wlif.yaml"
grep -q 'mountPath: "/var/run/secrets/tokens"' "$workdir/wlif.yaml"
if grep -q '^kind: Secret$' "$workdir/wlif.yaml" || grep -q 'name: sa-key' "$workdir/wlif.yaml"; then
  echo "WLIF mode rendered authorized-key resources" >&2
  exit 1
fi

if helm template scaler "$chart" \
  --set auth.workloadIdentityFederation.serviceAccountID=wlif-service-account \
  >"$workdir/wlif-missing-audience.yaml" 2>"$workdir/wlif-missing-audience.err"; then
  echo "WLIF mode rendered without an audience" >&2
  exit 1
fi
