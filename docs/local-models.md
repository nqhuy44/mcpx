# Local Models

Running LLMs locally eliminates cloud token costs for high-volume, repetitive tasks and keeps sensitive code/logs off the internet. This document covers model selection and inference setup for the two target hardware configurations in this project.

## Target Hardware

### Mac M1 / M4 (Apple Silicon)

Apple Silicon uses unified memory — VRAM and RAM are the same pool. The constraint is total system RAM and memory bandwidth.

| Chip | Typical RAM | Memory Bandwidth | Notes |
|---|---|---|---|
| M1 (base) | 8 GB | 68.25 GB/s | Very tight — 7B models only |
| M1 Pro/Max | 16–32 GB | 200–400 GB/s | Comfortable up to 13B, can squeeze 27B Q4 |
| M4 (base) | 16 GB | 120 GB/s | Good for 13B, 27B at Q4 is usable |
| M4 Pro/Max | 24–64 GB | 273–546 GB/s | 27B–35B at Q8, very fast inference |

Inference engine: **Ollama** (simplest) or **llama.cpp** (more control, lower overhead).

### RTX 3060 12GB (Windows/Linux)

The 3060 has 12GB GDDR6 VRAM. VRAM is the hard ceiling — model weights must fit entirely or it offloads to RAM, which kills performance.

| Quantization | Max model size in 12GB VRAM |
|---|---|
| F16 (full) | ~6B params |
| Q8 | ~12B params |
| Q6_K | ~16B params |
| Q4_K_M | ~24B params |
| Q3_K_M | ~32B params |

Inference engine: **Ollama** (GPU auto-detected on CUDA) or **vLLM** (for production serving with batching).

---

## Model Recommendations

### Primary: Qwen 3.6 (Best for Coding & DevOps)

Recommended as the default model for all coding, DevOps, and log-analysis tasks.

| Hardware | Variant | Quantization | Expected Speed | VRAM / RAM Used |
|---|---|---|---|---|
| M1 8GB | Qwen3-7B | Q4_K_M | ~25–35 tok/s | ~5 GB |
| M1 16GB / M4 16GB | Qwen3-14B | Q4_K_M | ~20–30 tok/s | ~9 GB |
| M1 Pro/Max 32GB | Qwen3-30B-A3B | Q4_K_M | ~18–25 tok/s | ~20 GB |
| M4 Pro/Max 24GB+ | Qwen3-32B | Q4_K_M | ~30–50 tok/s | ~22 GB |
| RTX 3060 12GB | Qwen3-14B | Q6_K | ~20–35 tok/s | ~11.5 GB |
| RTX 3060 12GB | Qwen3-7B | Q8 | ~45–60 tok/s | ~8 GB |

**Why Qwen 3.6:**
- 1M token context window (critical for large codebase/log analysis)
- Extremely resilient to quantization — cognitive quality stays high at Q4
- Best performance-per-watt on Apple Silicon
- Strong multilingual and code generation capabilities
- MIT licensed (no commercial restrictions)

**Qwen 3.6 Limitations:**
- Occasionally loose output formatting (mitigate with strict TOON/LEAN schema enforcement)
- 7B variant loses significant reasoning capability vs 14B+ — use for formatting and summarization only

### Alternative: Gemma 4 27B (Strict Formatting Tasks)

Use only when output format precision is the top priority (e.g., generating exact JSON schemas, coordinate-precise outputs).

| Hardware | Variant | Quantization | Expected Speed |
|---|---|---|---|
| M1 Pro 32GB | Gemma4-27B | Q4_K_M | ~18–22 tok/s |
| M4 Max 48GB+ | Gemma4-27B | Q8 | ~30+ tok/s |
| RTX 3060 12GB | Not viable | Q3 degrades severely | — |

**Avoid Gemma 4 on RTX 3060**: aggressive quantization required to fit 12GB VRAM severely degrades output quality — it's one of Gemma 4's known weaknesses. Use Qwen instead.

**Avoid Gemma 4 on M1 base (8GB)**: the model simply doesn't fit.

### Embedding Model (for Codebase Indexer & Tool Search)

| Hardware | Model | Speed | Dimension |
|---|---|---|---|
| All | `nomic-embed-text` | Very fast, tiny | 768 |
| All | `mxbai-embed-large` | Fast | 1024 |
| M4 / RTX 3060 | `bge-m3` | Moderate, multilingual | 1024 |

Embedding models run alongside the main LLM without significant RAM contention. Use `nomic-embed-text` by default; it's fast enough for real-time codebase indexing.

---

## Inference Engine Setup

### Ollama (Recommended for Local Dev)

The fastest path to running models locally. Works on Mac (Metal) and Linux/Windows (CUDA/ROCm).

```bash
# Install
curl -fsSL https://ollama.com/install.sh | sh

# Pull models
ollama pull qwen3:14b         # ~9GB download
ollama pull qwen3:7b          # ~5GB download
ollama pull nomic-embed-text  # ~274MB

# Start server (runs on :11434 by default)
ollama serve

# Test
ollama run qwen3:14b "Write a Go function to parse a JSON log line"
```

For RTX 3060, Ollama auto-detects CUDA. Verify with:
```bash
ollama run qwen3:14b --verbose
# Look for "using CUDA" in output
```

### llama.cpp (Lower Overhead, More Control)

Prefer llama.cpp when you need fine-grained control over context size, batch size, or GGUF quantization selection.

```bash
# Build (Mac)
brew install llama.cpp

# Build (Linux with CUDA)
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
cmake -B build -DGGML_CUDA=ON
cmake --build build --config Release -j

# Download GGUF model from HuggingFace
# e.g., Qwen/Qwen3-14B-GGUF → qwen3-14b-q4_k_m.gguf

# Run server
./build/bin/llama-server \
  -m models/qwen3-14b-q4_k_m.gguf \
  --ctx-size 32768 \
  --n-gpu-layers 99 \     # push all layers to GPU (RTX 3060)
  --host 0.0.0.0 \
  --port 8080
```

For Mac Metal (no `--n-gpu-layers` needed — Metal is auto-detected):
```bash
./build/bin/llama-server \
  -m models/qwen3-14b-q4_k_m.gguf \
  --ctx-size 131072 \
  --host 127.0.0.1 \
  --port 8080
```

### vLLM (Team / High-Throughput Server)

Use vLLM when serving the local model to multiple developers simultaneously, or when batching many agent requests.

```bash
pip install vllm

# Serve Qwen 3.6 14B on RTX 3060 with 12GB VRAM
python -m vllm.entrypoints.openai.api_server \
  --model Qwen/Qwen3-14B-Instruct \
  --quantization awq \
  --max-model-len 32768 \
  --gpu-memory-utilization 0.90 \
  --port 8000
```

vLLM exposes an OpenAI-compatible API. The mcpx `servers/llm` Python server wraps this endpoint.

---

## Integration with mcpx

The `servers/llm` MCP server wraps whichever inference engine is running locally. Configure via environment variable:

```bash
# Use Ollama (default)
MCPX_LLM_BACKEND=ollama
MCPX_LLM_OLLAMA_URL=http://localhost:11434
MCPX_LLM_DEFAULT_MODEL=qwen3:14b
MCPX_LLM_EMBED_MODEL=nomic-embed-text

# Use llama.cpp server
MCPX_LLM_BACKEND=llamacpp
MCPX_LLM_LLAMACPP_URL=http://localhost:8080

# Use vLLM
MCPX_LLM_BACKEND=vllm
MCPX_LLM_VLLM_URL=http://localhost:8000
```

The cloud LLM (Claude) routes tasks to `servers/llm` via the same MCP tool interface it uses for everything else. The backend swap is transparent.

---

## Task Routing Guide

Match tasks to model size based on reasoning demand:

| Task | Min Model | Reasoning demand |
|---|---|---|
| Format/transform data (JSON→LEAN) | 7B | Low |
| Summarize log output | 7B–14B | Low |
| Generate unit test stubs | 7B–14B | Low–Medium |
| Semantic code search ranking | 14B | Medium |
| Analyze CI failure root cause | 14B | Medium |
| Generate a migration script | 14B–32B | Medium–High |
| Cross-file architectural reasoning | Cloud (Claude) | High |
| Complex orchestration decisions | Cloud (Claude) | High |

7B on RTX 3060 at Q8 gets ~50 tok/s — fast enough for interactive use. 14B at Q6_K on 3060 is slightly slower (~25–35 tok/s) but significantly better reasoning.

---

## Context Window Strategy

Qwen 3.6 supports 1M token context, but local hardware caps effective usage:

| Hardware | Practical Context Limit | Notes |
|---|---|---|
| M1 8GB / Qwen3-7B | 8K–16K | Small tasks only |
| M1 16GB / Qwen3-14B | 32K–64K | Most SDLC tasks |
| RTX 3060 / Qwen3-14B Q6_K | 16K–32K | VRAM limits KV cache |
| M4 Max / Qwen3-32B | 128K–256K | Repository-level analysis |

For tasks that exceed local context capacity, the server must pre-filter (server-side) before sending to the local model. If the filtered input still exceeds capacity, route to the cloud model instead.
