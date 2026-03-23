# Maestro Kind Cluster — Initial Sizing Reference

> Measured: 2026-03-16
> Environment: WSL2 (Linux 6.6.87.2-microsoft-standard-WSL2), Podman, Kind
> Kubernetes: v1.35.0 / containerd 2.2.0

## Cluster Topology

Three kind clusters are deployed:

| Cluster | Role | Nodes |
|---|---|---|
| `maestro` | Server + DB + Agent (management) | 1 control-plane |
| `maestro-client-1` | Workload target | 1 control-plane |
| `maestro-client-2` | Workload target | 1 control-plane |

Each cluster node runs as an individual Podman container with its own isolated filesystem and containerd instance.

---

## Maestro Server Cluster (`maestro`)

### Pods

| Namespace | Pod | Role |
|---|---|---|
| `kube-system` | apiserver, etcd, controller-manager, scheduler, coredns ×2, kindnet, kube-proxy | Kubernetes control-plane |
| `local-path-storage` | local-path-provisioner | PV provisioner |
| `maestro` | `maestro` | Maestro server |
| `maestro` | `maestro-db` | PostgreSQL database |
| `maestro-agent` | `maestro-agent` | Maestro agent |

### CPU (idle)

| Metric | Value |
|---|---|
| Node capacity | 12 vCPUs |
| Pod requests | 1150m (9%) |
| Pod limits | 1100m (9%) |
| Actual usage (idle) | ~0% |

### Memory (idle)

| Metric | Value |
|---|---|
| Node capacity | ~15.5 GiB |
| Pod requests | 802 MiB (5%) |
| Pod limits | 1414 MiB (8%) |
| Actual container RSS | **~1.6 GiB** (9.65% of host RAM) |

### Disk

| Location | Usage | Notes |
|---|---|---|
| `/var/lib/containerd` | 2.7 GiB | Per-cluster; images + container layers |
| `/var/lib/etcd` | 125 MiB | Kubernetes state |
| `/var/log` | 12 MiB | System logs |
| Overlay root (`/`) | 117 GiB used / 251 GiB total | Shared with WSL2 host filesystem |

> **Note:** The overlay root is shared with the WSL2 host. Cluster-specific disk footprint is approximately **2.8 GiB** (containerd + etcd).
> **TODO:** Confirm whether containerd storage is isolated per cluster or shared across clusters.

---

## Open Questions

- [ ] Is containerd storage per-cluster or shared across all kind clusters?
- [ ] What is the idle memory footprint of `maestro-client-1` and `maestro-client-2`?
- [ ] Are resource requests/limits set on the `maestro` and `maestro-db` pods?
- [ ] What is the expected memory growth under load (e.g. 1k, 10k, 100k ManifestWorks)?
