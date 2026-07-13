#!/usr/bin/env bash
set -euo pipefail

chart="${CHART_DIR:-helm/yc-keda-external-scaler}"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

helm template yc-keda-external-scaler "$chart" \
  --set-string secret.data=test-key >"$workdir/default.yaml"
grep -q '^kind: Secret$' "$workdir/default.yaml"
grep -q 'automountServiceAccountToken: false' "$workdir/default.yaml"
grep -q 'image: "cr.yandex/sol/keda/yc-keda-external-scaler:v1.3.0"' "$workdir/default.yaml"
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
