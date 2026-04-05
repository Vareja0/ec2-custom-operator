#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-ec2-operator}"
RELEASE_NAME="${RELEASE_NAME:-ec2-operator}"
CHART_DIR="${CHART_DIR:-dist/chart}"
ACTION="${1:-}"

if [[ "$ACTION" != "create" && "$ACTION" != "delete" ]]; then
  echo "Usage: $0 <create|delete>"
  exit 1
fi

if [[ "$ACTION" == "create" ]]; then
  echo "==> Installing Prometheus Operator CRDs..."
  kubectl apply --server-side -f https://github.com/prometheus-operator/prometheus-operator/releases/latest/download/bundle.yaml

  echo "==> Waiting for Prometheus Operator to be ready..."
  kubectl wait --for=condition=Available deployment/prometheus-operator \
    --namespace default \
    --timeout=120s

  echo "==> Installing cert-manager..."
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
  kubectl wait --for=condition=Available deployment/cert-manager \
    --namespace cert-manager \
    --timeout=120s
  kubectl wait --for=condition=Available deployment/cert-manager-webhook \
    --namespace cert-manager \
    --timeout=120s
  echo "==> Waiting for cert-manager webhook to be fully ready..."
  kubectl wait --for=condition=Ready pod \
    --selector=app.kubernetes.io/component=webhook \
    --namespace cert-manager \
    --timeout=120s
  echo "==> Waiting for cert-manager cainjector to populate webhook caBundle..."
  for i in $(seq 1 30); do
    bundle=$(kubectl get validatingwebhookconfiguration cert-manager-webhook \
      -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null)
    if [[ -n "$bundle" ]]; then
      echo "    caBundle is ready."
      break
    fi
    echo "    Attempt $i/30 — waiting 5s..."
    sleep 5
  done

  echo "==> Creating monitoring namespace..."
  kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

  echo "==> Installing Helm chart..."
  helm upgrade --install "$RELEASE_NAME" "$CHART_DIR" \
    --namespace "$NAMESPACE" \
    --create-namespace \
    --values "$CHART_DIR/values.yaml" \
    --force-conflicts

  echo "==> Installing Grafana..."
  helm repo add grafana https://grafana.github.io/helm-charts 2>/dev/null || true
  helm repo update grafana
  helm upgrade --install grafana grafana/grafana \
    --namespace monitoring \
    --create-namespace \
    --values dist/grafana-values.yaml

  echo "==> Done. Release '$RELEASE_NAME' deployed to namespace '$NAMESPACE'."
  echo "==> Grafana deployed to namespace 'monitoring'."
  echo "==> Access Grafana: kubectl port-forward svc/grafana 3000:80 -n monitoring"
fi

if [[ "$ACTION" == "delete" ]]; then
  echo "==> Uninstalling Grafana..."
  helm uninstall grafana --namespace monitoring 2>/dev/null || true

  echo "==> Uninstalling Helm chart..."
  helm uninstall "$RELEASE_NAME" --namespace "$NAMESPACE" 2>/dev/null || true

  echo "==> Deleting namespaces..."
  kubectl delete namespace "$NAMESPACE" --ignore-not-found
  kubectl delete namespace monitoring --ignore-not-found

  echo "==> Removing Prometheus Operator CRDs..."
  kubectl delete -f https://github.com/prometheus-operator/prometheus-operator/releases/latest/download/bundle.yaml --ignore-not-found

  echo "==> Removing cert-manager..."
  kubectl delete -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml --ignore-not-found

  echo "==> Done. All resources removed."
fi
