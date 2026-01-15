# GitHub Copilot Instructions for MIT 6.5840 (Distributed Systems) Labs

This repository contains the Go source code for the MIT 6.5840 Distributed Systems labs. It is a monorepo structure where all labs share common infrastructure.

## High Level Details

- **Type**: Distributed Systems Course Labs.
- **Language**: Go 1.22+.
- **Module Root**: `src/` (contains `go.mod`).
- **Core Components**: MapReduce, Raft Consensus, Key-Value Store, Sharded KV Store.

## Build and Validation

There is no single global build command. Each lab is built and tested independently. The `Makefile` in the root is primarily for packaging submissions and ensuring no forbidden files are modified; it does **not** run the test suite for correctness.

**Important**: Always run Go commands from within the `src/` directory or its subdirectories.

### Lab 1: MapReduce
- **Code Location**: `src/mr/` (Coordinator/Worker logic), `src/main/` (Entry points), `src/mrapps/` (Plugins).
- **Validation**:
  1. Navigate to `src/main`.
  2. Run the test script: `bash test-mr.sh`.
  3. This script handles building plugins (`.so` files) and the main binaries (`mrcoordinator`, `mrworker`) and running the tests against them.
  - **Do NOT** rely on `go test ./...` for Lab 1, as the harness is a shell script.

### Lab 2: Key/Value Server (Single Node)
- **Code Location**: `src/kvsrv1/`.
- **Validation**:
  1. Navigate to `src/kvsrv1`.
  2. Run consistency tests: `go test -v`.
  - Recommended: Enable race detector: `go test -race -v`.

### Lab 3: Raft
- **Code Location**: `src/raft1/`.
- **Key File**: `src/raft1/raft.go` (implement Raft struct and logic here).
- **Validation**:
  1. Navigate to `src/raft1`.
  2. Run all Raft tests: `go test -v`.
  3. Run a specific test: `go test -v -run TestName`.
  4. Note: Tests are notoriously time-sensitive.
  - Recommended: `go test -race -v`.

### Lab 4: Fault-Tolerant Key/Value Service (Raft-based)
- **Code Location**: `src/kvraft1/`.
- **Validation**:
  1. Navigate to `src/kvraft1`.
  2. Run tests: `go test -v -race`.

### Lab 5: Sharded Key/Value Service
- **Code Location**: `src/shardkv1/`.
- **Validation**:
  1. Navigate to `src/shardkv1`.
  2. Run tests: `go test -v -race`.

### Common Build Flags
- `-race`: Usage is highly recommended to catch concurrency bugs.
- `-timeout`: Tests may deadlock. Default Go timeout is 10m.

## Project Layout

The repository is structured around a `src` directory containing the Go module.

- **`src/go.mod`**: Defines the module `6.5840`.
- **`src/labrpc/`**: Network RPC simulation library. **Do not modify.**
- **`src/labgob/`**: GOB helper library. **Do not modify.**
- **`src/models1/`**: Shared data models.
- **`src/tester1/`**: Shared testing infrastructure.

### Lab Directories
- **`src/mr/`**: MapReduce library (Lab 1).
  - `coordinator.go`: Coordinator implementation.
  - `worker.go`: Worker implementation.
  - `rpc.go`: RPC definitions.
- **`src/raft1/`**: Raft consensus library (Lab 3).
  - `raft.go`: Main Raft logic (RequestVote, AppendEntries, log replication).
  - `util.go`: Logging debug tools (`DPrintf`).
- **`src/kvsrv1/`**: Single-server KV (Lab 2).
- **`src/kvraft1/`**: Raft-replicated KV (Lab 4).
  - `client.go`: Clerk implementation.
  - `server.go`: KVServer implementation.
- **`src/shardkv1/`**: Sharded KV (Lab 5).
  - `shardctrler/`: Shard controller (config server) implementation.

## Tips for the Agent

1. **Debugging**: Use `DPrintf` (found in `util.go` or similar) for debug logging. It respects environment variables (usually `VERBOSE=1` or similar, check `util.go`).
2. **Concurrency**: This is a distributed systems course. Always consider race conditions. Use `sync.Mutex`.
3. **RPCs**:
   - Start with defining arguments and reply structs in `rpc.go` (or shared file).
   - Implement the RPC handler method.
   - Add the RPC call in the client/sender.
4. **Testing**:
   - The tests use a custom test harness (`tester1` package).
   - Failures often manifest as timeouts or "expected X got Y".
   - `porcupine` visualizer HTML files are generated on failure (e.g., `/tmp/porcupine-*.html`).
