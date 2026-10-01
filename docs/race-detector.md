# Go Race Detector

Concurrency issues like data races are notoriously difficult to track down because they depend on thread scheduling, hardware interleaving, and timing quirks. Go includes an integrated race detector based on ThreadSanitizer, accessible via the `-race` flag.

---

## 1. Core Mental Models

### Mental Model 1: Observer vs. Path Generator

> **Tests tạo ra execution path; race detector đứng quan sát xem nhiều goroutine có đụng shared memory sai cách hay không.**

```text
Tests
  ↓
run concurrent code
  ↓
Race Detector
  ↓
unsafe shared access?
├─ yes → report race
└─ no  → pass
```

### Mental Model 2: Không phải Unit Test

```text
go test
→ behavior đúng không? (kiểm tra logic / assertions)

-race
→ concurrent memory access có an toàn không? (phân tích xung đột bộ nhớ runtime)
```

Nên:
```text
go test -race ./...
=
run tests (chạy test suites)
+
enable runtime race detection (bật cơ chế phát hiện data race khi chạy)
```

### Mental Model 3: The Three Pillars of Concurrency Safety

```text
Tests
  ↓ (generate execution paths)
Race Detector
  ↓ (watches memory accesses on those paths)
Synchronization (Mutex, Atomic, Channels)
  ↓ (makes shared state safe)
```

> **Rút gọn trong một câu:**
> **Tests exercise concurrency; the race detector checks whether that execution accesses shared memory safely.**

---

## 2. Dynamic Nature and Limitations

> **Điểm quan trọng nhất: Race detector chỉ thấy race ở những code path thực sự được chạy.**

```text
race exists
     +
test không chạm tới
     ↓
detector không thấy
```

```text
data race exists in codebase
           +
code path never executed during test run
           ↓
Race detector CANNOT report it
```

### Key Takeaways:
1. **Absence of a race report is not mathematical proof of correctness**: A green `-race` run only proves that no unsynchronized accesses occurred along the *specifically executed code paths*.
2. **Workload generation matters**: To catch concurrency bugs in shared components, tests must explicitly spawn concurrent goroutines exercising shared mutable state.

---

## 3. Concurrency-Sensitive Targets in the Project

The project contains several components that require strict synchronization:
- [`FixedWindowLimiter`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/limiter.go#L32):
  - In-memory `clients` map accessed concurrently by incoming HTTP handlers.
  - Background eviction goroutine running periodically to clean expired keys.
  - Quota counters (`count`) updated per client.
- Database connection pools (`pgxpool.Pool`).
- Cache-aside layer with Redis or in-memory stores.

### Concurrency Test Example: `FixedWindowLimiter`

Sequential tests do not provide the race detector with concurrent memory access patterns. In [`internal/gateway/limiter_test.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/limiter_test.go), multiple tests feed concurrent workloads into the detector:
1. `TestFixedWindowLimiter_ConcurrentAllow_SingleKey`: Spawns 100 goroutines concurrently calling `Allow()` for the same client key.
2. `TestFixedWindowLimiter_ConcurrentAllow_MultipleKeys`: Spawns concurrent goroutines across distinct client keys, stressing map allocations and mutations.
3. `TestFixedWindowLimiter_ConcurrentAllowAndStop`: Exercises concurrent requests while concurrently terminating the background eviction cleaner via `Stop()`.

---

## 4. Local Workflow

### Fast Tests vs. Race Tests

The race detector adds ~2-10x CPU overhead and ~5-20x memory overhead because the compiler instruments 8-byte synchronization shadow memory for each memory access. Keep fast tests available for quick iteration:

```bash
# Fast local test run (no instrumentation overhead)
make test
# Or: go test -v ./...

# Race-instrumented test run (for concurrency-sensitive packages)
make test-race
# Or: go test -v -race ./...

# Target a specific package
go test -v -race ./internal/gateway/...
```

---

## 5. CI Integration

The race detector is integrated directly into the GitHub Actions CI pipeline ([`.github/workflows/audit.yml`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/.github/workflows/audit.yml)):

```text
Push / PR
   ↓
Checkout & Setup Go
   ↓
go mod verify
   ↓
go build -v ./...
   ↓
go vet ./... (static analysis)
   ↓
go test -v -race ./... (concurrency analysis)
   ↓
Pass / Fail
```

Any detected race immediately fails the CI run with full stack traces of the competing goroutines.

---

## 6. Guardrails

1. **Never treat `-race` as a replacement for careful synchronization**: Design concurrency safely using mutexes, atomics, or channels first.
2. **Never remove synchronization because tests pass**: Unsynchronized access might just fail to collide during test execution.
3. **Fail CI immediately on race detection**: Treat race warnings as critical build failures, never as ignorable warnings.
4. **Target shared mutable state**: When writing components with background loops, shared caches, or maps, accompany them with concurrent stress tests.
