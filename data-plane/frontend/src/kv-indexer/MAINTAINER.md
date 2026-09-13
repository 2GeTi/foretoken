<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# KV Index Maintenance

English | [中文](MAINTAINER_zh.md)

The KV index combines local cache events with live shared-store observations for Router scoring. Cache storage and movement remain with the backend.

## Protocol boundary

Each event source is isolated by model revision, KV scope, route target, and data-parallel rank. Event streams use epochs and continuous cursors. Missing, reordered, or incompatible events must degrade the source to `Unavailable` rather than expose a false locality match.

Production route bindings expose target-local `Device/Local` event matches. Mooncake shared-prefix observations use a separate request-time path: the indexer asks a healthy model-server, which calls the active connector over Pod-local IPC. The adapter reuses vLLM's native hashing, key namespaces, and complete-shard lookup against the existing Store handle. Negative Store results remain unavailable rather than becoming misses.

The controller publishes a query-sharing scope from the managed KVService UID; external profiles use a ModelGroup-local scope. The indexer combines this with the existing model/layout KV scope, queries each compatible scope once, and retains results only for the current routing request. Shared hits use `External/Remote`; no Store inventory or event history is reconstructed. A shared-query failure preserves valid local matches, but an unknown external result cannot turn an empty local observation into a confirmed miss.

## Index resolution

`PositionalHashIndex` and `RadixTreeIndex` are internal implementations selected by topology-aware configuration. `NoopKvPrefixIndexer` is a Router no-op used when locality is unavailable. None is a user-selectable `FrontendService` setting.

## Backend adapters

Backend-specific block identifiers, event lifecycles, and placement semantics belong in a backend adapter. Preserve exact event-source identity and sequence rules when extending an adapter. A backend must publish enough fidelity for the index to distinguish confirmed matches, confirmed misses, and unavailable observations.

The Mooncake adapter uses the existing controller KV scope as its default native `cache_prefix`, preserving an explicit connector prefix. The default namespace changes with the configured model revision and layout, and is shared by actual transfers and lookup. The engine image installs this thin connector module alongside model-server; no upstream source patch or additional Store client is required.

When changing the protocol or index behavior, update the user-facing KV guide, Router guide, runtime diagnostics, metrics, and contract tests together.
