#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-ec2-operator}"
RELEASE_NAME="${RELEASE_NAME:-ec2-operator}"
CHART_DIR="${CHART_DIR:-dist/chart}"

echo "==> Installing Prometheus Operator CRDs..."
kubectl apply --server-side -f https://github.com/prometheus-operator/prometheus-operator/releases/latest/download/bundle.yaml

echo "==> Waiting for Prometheus Operator to be ready..."
kubectl wait --for=condition=Available deployment/prometheus-operator \
  --namespace default \
  --timeout=120s

echo "==> Installing Helm chart..."
helm upgrade --install "$RELEASE_NAME" "$CHART_DIR" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --values "$CHART_DIR/values.yaml"

echo "==> Done. Release '$RELEASE_NAME' deployed to namespace '$NAMESPACE'."
