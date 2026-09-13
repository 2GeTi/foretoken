<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# KV Prefix Index

The KV prefix index helps the Router reuse work from earlier requests. It combines observations of each target's local accelerator cache with live shared-prefix queries through Mooncake Store connectors. Cache storage, eviction, and transfers remain with the inference backend.

Shared queries support single-DP, plain-token requests. Compatible targets using the same managed Store share one query per routing request. Store memory and SSD hits are reported as shared external cache.

With the [`kv_least_loaded` routing scorer](../router/README.md), lookup results affect selection as follows:

- **Match:** prefer longer cached prefixes; for equal lengths, prefer the local accelerator cache over shared storage, then compare load.
- **Miss:** no reusable prefix was found for that target, so it receives no cache preference.
- **`Unavailable`:** the index cannot give a reliable answer. This is not a miss; the target receives no cache preference, but remains eligible for ordinary routing.

Targets must still be healthy and compatible with the request. A match makes reuse more likely, but the backend may evict the cache before execution begins.

## Operations

Use the frontend's `/statusz` endpoint to inspect KV-index health and any degradation reason, and `/metrics` for Prometheus monitoring. If the index remains degraded, use the reported reason to investigate model-server cache updates. See [frontend endpoint access](../../README.md#endpoint-access).

Protocol details and backend integration are covered in [KV index maintenance](MAINTAINER.md).
