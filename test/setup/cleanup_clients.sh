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

# Cleans up all Maestro client kind clusters created by add_clients.sh.
# Deletes every kind cluster named 'maestro-client-*' and removes their output files.
#
# Usage:
#   ./test/setup/cleanup_clients.sh

if ! command -v kind >/dev/null 2>&1; then
    echo "kind not found, skipping cluster deletion"
else
    for cluster in $(kind get clusters 2>/dev/null | grep '^maestro-client-' || true); do
        kind delete cluster --name "${cluster}"
    done
fi

# Remove client-specific output files
rm -f ${PWD}/test/_output/.kubeconfig-maestro-client-*
rm -f ${PWD}/test/_output/.consumer_name-maestro-client-*
rm -f ${PWD}/test/_output/maestro-agent-values-maestro-client-*.yaml
