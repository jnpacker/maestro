#!/bin/bash -ex
#
# Deploy the managedcluster-soloist-controller (server mode) into the
# test-env KinD cluster. Assumes Maestro is already deployed and reachable
# at the in-cluster service addresses configured in the soloist ConfigMap.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAESTRO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

export KUBECONFIG=${MAESTRO_ROOT}/test/_output/.kubeconfig

# Allow overriding the soloist source directory
SOLOIST_DIR="${SOLOIST_DIR:-$(cd "${MAESTRO_ROOT}/../soloist" && pwd)}"

echo "==> Deploying managedcluster-soloist-controller from ${SOLOIST_DIR}"

# Install the ManagedCluster CRD required by the controller
kubectl apply -f "${SOLOIST_DIR}/deploy/crds/managedclusters.crd.yaml"

# Apply the server-mode kustomization (namespace, RBAC, ConfigMap, Deployment)
kubectl kustomize "${SOLOIST_DIR}/deploy/server" | \
  sed 's|quay.io/jpacker/managedcluster-soloist-controller[^"]*|quay.io/jpacker/soloist:latest|g' | \
  kubectl apply -f -

# Wait for the deployment to be ready
kubectl wait deploy/managedcluster-soloist-controller \
  -n open-cluster-management \
  --for condition=Available=True \
  --timeout=300s

echo "==> managedcluster-soloist-controller deployed successfully"
