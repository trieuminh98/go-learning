# Release Automation with Conventional Commits

## 1. Core Mental Models

### Mental Model 1: Machine-Readable Commit Intent

> **Commit message không chỉ để người đọc — nó trở thành machine-readable intent để hệ thống tự suy ra version và changelog.**

```text
Commit intent
   ↓
Conventional Commits
   ↓
Release automation (Release Please)
   ↓
Version bump + CHANGELOG
   ↓
Release PR
   ↓
Human approves
```

### Mental Model 2: Machine Preparation vs. Human Release Gate

> **Automation chuẩn bị release; human vẫn giữ quyền quyết định release.**

```text
Machine
→ calculate version
→ generate changelog
→ open Release PR

Human
→ review
→ merge
```

Hệ thống cố ý giữ Release PR để merge thủ công thay vì auto-merge. Máy móc làm các tác vụ lặp đi lặp lại (tính toán SemVer, trích xuất commit, viết changelog), nhưng con người nắm quyền kiểm soát phát hành cuối cùng.

### Mental Model 3: Single Source of Truth

> **Version nên có một source of truth, còn các nơi khác derive từ đó.**

```text
release version (.release-please-manifest.json / tag)
      ↓
source code
docs (Swagger / OpenAPI)
metrics (/debug/vars)
deployment metadata
```

Nếu duplicate version ở nhiều nơi (trong code, trong doc, trong release), khi version release đã đổi nhưng version trong code vẫn cũ, ta sẽ phải thêm automation phụ trợ chỉ để đi sync lại. Thiết kế đúng là để toàn bộ runtime derive từ một nguồn phát hành duy nhất.

> **Rút gọn trong đúng một câu:**  
> **Encode release intent once in commits, let automation derive version + changelog, then keep a human gate before release.**

---

## 2. Conventional Commits as the Release Contract

Commit messages serve as a machine-readable contract that directly dictates Semantic Versioning:

```text
fix:
→ PATCH bump (e.g., 1.0.0 → 1.0.1)

feat:
→ MINOR bump (e.g., 1.0.0 → 1.1.0)

BREAKING CHANGE / type!:
→ MAJOR bump (e.g., 1.0.0 → 2.0.0)
```

### Examples
- `fix: correct feed pagination cursor` → Patch release
- `feat: implement user filtering by status` → Minor release
- `feat!: change authentication response format` → Major release

> **Important Boundary:** Inconsistent or non-standard commit messages break release automation. Conventional Commits are not merely stylistic; they are part of the delivery pipeline contract.

---

## 3. Architecture & Release Please Flow

Release Please is configured via GitHub Actions and two manifest files in the repository root:

```text
Repository Root
├── .github/workflows/release.yml    # Triggers on push to main
├── release-please-config.json       # Configures Go package and changelog settings
├── .release-please-manifest.json    # Authoritative single source of truth for version
└── CHANGELOG.md                     # Cumulative release notes generated automatically
```

### Manifest Files
- **`release-please-config.json`**:
  Configures the release type as `go`, specifies `CHANGELOG.md`, and sets `include-component-in-tag: false` for clean `vX.Y.Z` tags.
- **`.release-please-manifest.json`**:
  Tracks the current project version (e.g., `{"." : "1.0.0"}`).

---

## 4. Single Source of Truth & Runtime Versioning

> **Duplicated version information creates synchronization debt.**

Thay vì hardcode version numbers rải rác:
1. **`.release-please-manifest.json`**: Là nguồn chân lý duy nhất cho version của dự án.
2. **Runtime Configuration**: API runtime đọc `Version` từ biến môi trường `APP_VERSION` (hoặc build ldflags), sau đó truyền vào các tầng cần thiết:
   - [`internal/api/health.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/api/health.go): Endpoint `/v1/health`.
   - [`internal/gateway/metrics.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/internal/gateway/metrics.go): Endpoint `/debug/vars`.
   - [`cmd/api/main.go`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go#L319): Gán động [`docs.SwaggerInfo.Version = cfg.Version`](file:///Users/minhtintrieu/Documents/documents/Personals/go-learning/cmd/api/main.go#L319).

---

## 5. End-to-End Delivery Pipeline

Release automation operates downstream of quality verification:

```text
Code Change
    ↓
CI Audit (.github/workflows/audit.yml)
├─ go mod verify
├─ go build
├─ go vet (static analysis)
└─ go test -v -race ./... (concurrency checks)
    ↓ [pass]
Merge to main
    ↓
Release Please (.github/workflows/release.yml)
    ↓
Open / Update Release PR
    ↓
Human Review & Merge
    ↓
GitHub Release Created & Tagged
```

---

## 6. Credentials & Guardrails

1. **Least-Privilege Token Permissions**: The release workflow requests only `contents: write` and `pull-requests: write`.
2. **Secret Storage**: If using a custom GitHub Personal Access Token (to trigger downstream workflows on PR creation), store it as `RELEASE_PLEASE_TOKEN` in GitHub Actions secrets. Never commit tokens.
3. **Squash Merge Caveat**: When squash-merging feature branches into `main`, ensure the squashed commit message retains Conventional Commit syntax (e.g., `feat: ...`), otherwise Release Please cannot deduce the version bump.
4. **No Auto-Merge**: The Release PR remains open until a human operator decides to merge it.
