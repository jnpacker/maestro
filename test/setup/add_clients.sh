#!/bin/bash -ex
#
# Copyright (c) 2023 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Creates N additional kind clusters and registers each as a Maestro consumer with a deployed agent.
# Requires an existing Maestro server deployed via 'make test-env/deploy-server'.
#
# Usage:
#   N_CLIENTS=3 ./test/setup/add_clients.sh

N_CLIENTS=${N_CLIENTS:-1}
tls_enable=${ENABLE_MAESTRO_TLS:-"false"}

export image_tag=${image_tag:-"latest"}
export external_image_registry=${external_image_registry:-"image-registry.testing"}

export namespace="maestro"
export agent_namespace="maestro-agent"
export SERVER_KUBECONFIG=${PWD}/test/_output/.kubeconfig

mkdir -p ${PWD}/test/_output

# Verify the server is deployed
restapi_endpoint=$(cat ${PWD}/test/_output/.external_restapi_endpoint 2>/dev/null || true)
if [ -z "$restapi_endpoint" ]; then
  echo "ERROR: REST API endpoint not found in test/_output/.external_restapi_endpoint" >&2
  echo "Run 'make test-env/deploy-server' before adding clients." >&2
  exit 1
fi

# Determine container tool
if command -v docker &> /dev/null; then
    container_tool="docker"
elif command -v podman &> /dev/null; then
    container_tool="podman"
else
    echo "Neither Docker nor Podman is installed, exiting"
    exit 1
fi

# Get the IP of the server cluster's control plane node.
# This IP is reachable from other kind clusters sharing the same Docker/Podman network.
server_node_ip=$(kubectl --kubeconfig ${SERVER_KUBECONFIG} get nodes \
  -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
if [ -z "$server_node_ip" ]; then
  echo "ERROR: Could not determine server cluster node IP" >&2
  exit 1
fi

# The gRPC broker is exposed via NodePort 30091 in the server cluster.
grpc_url="${server_node_ip}:30091"

for i in $(seq 1 ${N_CLIENTS}); do
  client_name="maestro-client-${i}"
  client_kubeconfig="${PWD}/test/_output/.kubeconfig-${client_name}"
  consumer_name_file="${PWD}/test/_output/.consumer_name-${client_name}"

  echo "==> Setting up ${client_name} (${i}/${N_CLIENTS})"

  # Create the kind cluster if it doesn't already exist
  if [ ! -f "$client_kubeconfig" ]; then
    cat << EOF | kind create cluster --name ${client_name} --kubeconfig ${client_kubeconfig} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    apiVersion: kubelet.config.k8s.io/v1beta1
    syncFrequency: "1s"
    configMapAndSecretChangeDetectionStrategy: "Watch"
EOF
  fi

  # Load the maestro image into the client cluster
  if [ "$container_tool" = "docker" ]; then
    kind load docker-image ${external_image_registry}/maestro/maestro:${image_tag} --name ${client_name}
  else
    podman save ${external_image_registry}/maestro/maestro:${image_tag} -o /tmp/maestro-${client_name}.tar
    kind load image-archive /tmp/maestro-${client_name}.tar --name ${client_name}
    rm /tmp/maestro-${client_name}.tar
  fi

  # Apply CRDs to the client cluster before the agent starts so they are available
  # when resource bundles containing these types arrive.
  # ManagedCluster CRD must be present before the agent deploys resources that use it;
  # if missing, the agent will retry with backoff but will report errors in the interim.
  kubectl --kubeconfig ${client_kubeconfig} apply \
    -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/cluster/v1/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml
  kubectl --kubeconfig ${client_kubeconfig} apply \
    -f https://raw.githubusercontent.com/open-cluster-management-io/api/release-0.14/work/v1/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml

  # Create the agent namespace
  kubectl --kubeconfig ${client_kubeconfig} create namespace ${agent_namespace} || true

  # Register a new consumer with the Maestro server
  if [ ! -f "${consumer_name_file}" ]; then
    response=$(curl -s -k -X POST -H "Content-Type: application/json" \
      "${restapi_endpoint}/api/maestro/v1/consumers" -d '{}')
    consumer_name=$(echo "$response" | jq -r '.id')
    if [ -z "$consumer_name" ] || [ "$consumer_name" = "null" ]; then
      echo "ERROR: Failed to create consumer for ${client_name}" >&2
      exit 1
    fi
    echo "$consumer_name" > ${consumer_name_file}
  fi
  consumer_name=$(cat ${consumer_name_file})

  # Build Helm values for this agent
  values_file="${PWD}/test/_output/maestro-agent-values-${client_name}.yaml"

  cat > "$values_file" <<EOF
environment: development

clientCertRefreshDuration: 5s

consumerName: ${consumer_name}

cloudeventsClientId: ${consumer_name:0:52}-work-agent

serviceAccount:
  name: maestro-agent-sa

image:
  registry: ${external_image_registry}
  repository: maestro/maestro
  tag: ${image_tag}
  pullPolicy: IfNotPresent

logging:
  klogV: "4"

messageBroker:
  type: grpc
  grpc:
    url: ${grpc_url}
EOF

  if [ "$tls_enable" = "true" ]; then
    cat >> "$values_file" <<EOF
    tls:
      caCert: /secrets/grpc-broker-cert/ca.crt
      clientCert: /secrets/grpc-broker-cert/client.crt
      clientKey: /secrets/grpc-broker-cert/client.key
EOF
  fi

  # Deploy the agent using Helm
  helm upgrade --install maestro-agent \
    ./charts/maestro-agent \
    --kubeconfig ${client_kubeconfig} \
    --namespace "${agent_namespace}" \
    --values "$values_file" \
    --wait \
    --timeout 5m

  kubectl --kubeconfig ${client_kubeconfig} wait deploy/maestro-agent \
    -n ${agent_namespace} --for condition=Available=True --timeout=200s

  # Deploy the soloist client into the client cluster
  soloist_deploy="${PWD}/../soloist/deploy"
  kubectl --kubeconfig ${client_kubeconfig} create namespace open-cluster-management-agent || true
  sed 's|quay.io/jpacker/managedcluster-soloist-controller[^"]*|quay.io/jpacker/soloist:latest|g' \
    "${soloist_deploy}/client/install.yaml" | \
    kubectl --kubeconfig ${client_kubeconfig} apply -f -
  kubectl --kubeconfig ${client_kubeconfig} wait deploy/managedcluster-soloist-client \
    -n open-cluster-management-agent --for condition=Available=True --timeout=200s

  echo "==> ${client_name} ready"
  echo "    consumer ID:  ${consumer_name}"
  echo "    kubeconfig:   ${client_kubeconfig}"
done

echo ""
echo "Successfully added ${N_CLIENTS} client(s)."
