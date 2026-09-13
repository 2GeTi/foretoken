<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- SPDX-FileCopyrightText: Copyright contributors to the Foretoken project -->

# KV 索引维护指南

[English](MAINTAINER.md) | 中文

KV 索引结合本地缓存事件与共享存储的实时观测，为 Router 评分提供依据。缓存存储和搬运仍由后端负责。

## 协议边界

每个事件源按模型 revision、KV scope、route target 和 data-parallel rank 隔离。事件流使用 epoch 和连续 cursor。事件缺失、乱序或不兼容时，事件源必须退化为 `Unavailable`，不能暴露错误的缓存位置匹配。

生产路由配置只使用事件索引中目标自身的 `Device/Local` 命中。Mooncake 共享前缀在请求时查询：索引器向健康的 model-server 发起查询，再由 Pod 内的只读 IPC 调用实际 connector。适配层复用 vLLM 原生哈希、缓存键和完整分片查询，不另建 Store 客户端；Store 负错误码保留为不可用，不转换成未命中。

控制器用托管 KVService UID 确定查询复用范围，外部 profile 则限定到单个 ModelGroup。索引器再结合已有模型与布局 KV scope，为每个兼容范围查询一次，结果仅在当前路由请求内保留。共享命中标记为 `External/Remote`，不维护 Store 全量目录或历史事件副本。外部查询失败时保留有效本地命中；本地为空且外部未知时返回 `Unavailable`。

## 索引解析

`PositionalHashIndex` 和 `RadixTreeIndex` 是由 topology-aware 配置选择的内部实现。`NoopKvPrefixIndexer` 是位置不可用时 Router 使用的 no-op 实现。它们都不是用户可配置的 `FrontendService` 选项。

## Backend adapter

后端专有的 block 标识、事件生命周期和 placement 语义属于 backend adapter。扩展 adapter 时必须保持精确的事件源身份和序号规则。后端需要提供足够完整的事件，使索引能够区分已确认匹配、已确认未命中和不可用观测。

Mooncake 适配层把控制器已有的 KV scope 用作默认原生 `cache_prefix`，保留显式配置的 connector 前缀。默认命名空间随配置的模型 revision 和布局变化，实际传输与查询共同使用它。引擎镜像随 model-server 安装这个薄 connector 模块，不修改上游源码，也不另建 Store 客户端。

修改协议或索引行为时，应同步更新面向用户的 KV 指南、Router 指南、runtime 诊断、指标和 contract tests。
