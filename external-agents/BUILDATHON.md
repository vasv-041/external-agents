# Track 3: Bring Entire to a New Agent or Workflow

**Project Name:** Entire Agent Integration for Qwen Code CLI (`entire-agent-qwen`)  
**Repository:** [vasv-041/external-agents](https://github.com/vasv-041/external-agents)  
**Track:** Track 3 — Bring Entire to a New Agent or Workflow  

---

## 1. Overview & Architecture

`entire-agent-qwen` is a dedicated external agent integration written in Go that connects Entire’s checkpointing protocol with **Qwen Code CLI**. It implements the full protocol lifecycle over stdin/stdout JSON channels to capture meaningful session metadata, file changes, tool activity, and lifecycle events.

---

## 2. Key Capabilities & Captured Context

* **Hook Lifecycle Management:** Registers and executes lifecycle hooks natively compatible with Qwen's `.qwen/settings.json` configuration.
* **Transcript & Session Analysis:** Parses raw Qwen CLI session logs from stdout and transcript directories to track prompt histories, tool executions, and file modifications.
* **Dual-Format & Safe Parsing:** Supports both legacy and current event structures with graceful fallback to output partial summary data for incomplete or unclosed sessions.
* **Checkpoint & Metadata Generation:** Automatically syncs captured developer actions directly into Entire checkpoint refs (`entire/checkpoints/v1`).

---

## 3. Verification & Testing

### Build the Binary
```cmd
cd agents/entire-agent-qwen
go build -o entire-agent-qwen.exe ./cmd/entire-agent-qwen