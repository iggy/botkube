# :red_circle: prod-eu

**Status:** Failing
**Kubernetes:** `v1.34.2`
**Nodes:** 2/3 ready
**Pods:** 42 total, 2 unhealthy
**Workloads:** 17 in 4 namespaces
**Updated:** Sat, 01 Aug 2026 12:30:00 UTC

## Nodes

| | Node | Ready | Schedulable | Kubelet | Reason |
|---|---|---|---|---|---|
| :large_green_circle: | node-1 | True | yes | `v1.34.2` |  |
| :red_circle: | node-2 | False | yes | `v1.34.2` | KubeletNotReady |
| :large_yellow_circle: | node-3 | True | no | `v1.34.1` | unschedulable |

## Unhealthy workloads

| | Kind | Namespace | Name | Ready | Restarts | Reason |
|---|---|---|---|---|---|---|
| :red_circle: | Pod | `shop` | api-7c9d-x2k | 0/1 | 12 | CrashLoopBackOff |
| :large_yellow_circle: | Deployment | `shop` | web | 2/3 |  | 2/3 ready |

## Service catalog

| | Workload | Namespace | Container | Version | Ready | Rollout |
|---|---|---|---|---|---|---|
| :large_green_circle: | api | `shop` | `api` | `1.4.2` | 3/3 | settled |
| | | | `sidecar` | `2.0.0` | | |
| :large_yellow_circle: | web | `shop` | `web` | `2.1.0` | 2/3 | :arrows_counterclockwise: in progress |

## Recent warnings

| Age | Object | Reason | Count | Message |
|---|---|---|---|---|
| Sat, 01 Aug 2026 12:30:00 UTC | shop/Pod/api-7c9d-x2k | BackOff | x12 | Back-off restarting failed container |
| Sat, 01 Aug 2026 12:28:00 UTC | shop/Pod/web-1 | Unhealthy | x1 | Readiness probe failed |

---

_Maintained by Botkube. Manual edits are overwritten on the next update._
