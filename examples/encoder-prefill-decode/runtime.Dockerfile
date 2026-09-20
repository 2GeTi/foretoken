# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the Foretoken project

# NVIDIA runtime with encoder-only execution and Mooncake P/D transfer.
FROM vllm/vllm-openai@sha256:4cbfd34aac145fd1870381c030131c7f868fcad45448f401ecdb5fd4ed020b42
RUN python3 -m pip install --no-cache-dir --only-binary=:all: \
        mooncake-transfer-engine==0.3.12.post1 nvidia-cuda-runtime-cu12 \
    && ln -s /usr/bin/python3 /usr/local/bin/python
ENV LD_LIBRARY_PATH=/usr/local/lib/python3.12/dist-packages/nvidia/cuda_runtime/lib:${LD_LIBRARY_PATH}
