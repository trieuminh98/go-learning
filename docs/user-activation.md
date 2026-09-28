# User Activation: One-Time Capability & Atomic State Transition

This document details the mental models, architectural boundaries, and implementation patterns for the user account activation workflow.

---

## 1. Core Mental Models

> **Mental core của User Activation**:
>
> *Activation là một one-time state transition được authorize bằng verification token.*

```text
plain token
   ↓ verify
valid + not expired?
   ↓
inactive user
   ↓
ACTIVE
   ↓
consume token
```

### 1.1. Bản chất của Activation

Điểm quan trọng là activation không chỉ đơn thuần là cập nhật cờ boolean:

```text
is_active = true
```

mà là một chuỗi hành động authorization và lifecycle:

```text
prove capability
→ change state
→ invalidate capability
```

### 1.2. Atomic State Transition Invariant

```text
BEGIN tx

invitation = tx.FindInvitation(tokenHash)
verify unexpired (expiry > now)
tx.ActivateUser(invitation.UserID)
tx.DeleteInvitation(invitation.UserID)

COMMIT / ROLLBACK
```

> [!IMPORTANT]
> Toàn bộ các bước kiểm tra token, cập nhật trạng thái user (`is_active = true`), và xoá token invitation phải thực thi trong **cùng một transaction boundary**.
>
> Source implement chuỗi `find invitation → activate user → delete invitation` trong cùng transaction để tránh rơi vào trạng thái nửa vời (partial state) hoặc tạo cửa sổ race condition cho phép replay token nhiều lần.

### 1.3. Verification Secret Partitioning & Direct Identity Resolution

```text
Client request: PUT /v1/authentication/users/activate/{token}
      ↓
Logic: tokenHash = SHA-256(plainToken)
      ↓
DB Lookup: SELECT ... WHERE token_hash = $1
      ↓
User ID resolved directly from invitation record
```

> [!CAUTION]
> Tuyệt đối **không tin tưởng hoặc yêu cầu `user_id` từ client**. Quan hệ giữa token hash và invitation record đã xác định danh tính user một cách không thể làm giả. Nếu client gửi kèm `user_id`, kẻ tấn công có thể thử brute-force hoặc bypass validation.

### 1.4. Rút gọn một câu để nhớ:

```text
Verify once, transition once, consume the proof.
```

---

## 2. Sequence Flow

```text
Client                               API                             Logic                       Database (Tx)
  │                                   │                                │                               │
  │─── PUT /users/activate/{token} ──>│                                │                               │
  │                                   │─── ActivateUser(token) ───────>│                               │
  │                                   │                                │─── SHA-256(token)             │
  │                                   │                                │─── WithinTransaction ────────>│
  │                                   │                                │                               │─── GetByTokenHash
  │                                   │                                │                               │─── Expiry check
  │                                   │                                │                               │─── ActivateUser (is_active=true)
  │                                   │                                │                               │─── DeleteByUserID (consume)
  │                                   │                                │<── COMMIT ────────────────────│
  │                                   │<── nil ────────────────────────│                               │
  │<── 204 No Content ────────────────│                                │                               │
```

---

## 3. Implementation Patterns

### 3.1. Database Query (`internal/db/query/users.sql`)

```sql
-- name: ActivateUser :exec
UPDATE users
SET is_active = true
WHERE id = $1;
```

### 3.2. Logic Orchestration (`internal/logic/user.go`)

```go
func (s *UserService) ActivateUser(ctx context.Context, plainToken string) error {
	plainToken = strings.TrimSpace(plainToken)
	if plainToken == "" {
		return core.ErrInvitationNotFound
	}

	tokenHash := core.HashToken(plainToken)

	return s.txManager.WithinTransaction(ctx, func(tx TxStorage) error {
		invitation, err := tx.Invitations.GetByTokenHash(ctx, tokenHash)
		if err != nil {
			return err
		}

		if time.Now().After(invitation.Expiry) {
			return core.ErrInvitationNotFound
		}

		if err := tx.Users.Activate(ctx, invitation.UserID); err != nil {
			return err
		}

		return tx.Invitations.DeleteByUserID(ctx, invitation.UserID)
	})
}
```

### 3.3. Uniform Error Contract (Anti-Enumeration)

| Condition | Domain Error | HTTP Status | Response Payload |
| :--- | :--- | :--- | :--- |
| Token does not exist | `core.ErrInvitationNotFound` | `404 Not Found` | `{"error": "invitation not found or expired"}` |
| Token expired (`now > expiry`) | `core.ErrInvitationNotFound` | `404 Not Found` | `{"error": "invitation not found or expired"}` |
| Token already consumed | `core.ErrInvitationNotFound` | `404 Not Found` | `{"error": "invitation not found or expired"}` |
| Valid & unexpired | `nil` | `204 No Content` | *(empty body)* |

> [!NOTE]
> Việc trả về cùng một mã lỗi `404 Not Found` với thông điệp đồng nhất cho cả ba trường hợp (không tồn tại, hết hạn, hoặc đã được kích hoạt trước đó) loại bỏ khả năng dò tìm token hợp lệ của kẻ tấn công (timing / enumeration attack oracle).

---

## 4. Architectural Guardrails

1. **No Client-Provided User ID**: Endpoint chỉ nhận `{token}` trên URL path.
2. **Hash-at-Rest**: Plaintext token chỉ được truyền qua wire và tính SHA-256 hash trên application memory, không bao giờ so sánh plain token trong SQL.
3. **Strict Expiration Check**: Luôn kiểm tra `time.Now().After(invitation.Expiry)` trước khi thực hiện kích hoạt user.
4. **Immediate Invalidation**: Token bị xoá ngay trong cùng transaction kích hoạt để triệt tiêu cửa sổ replay attacks.
5. **Driver Independence**: Logic chỉ phụ thuộc vào `logic.TransactionManager` và `logic.TxStorage`, không để rò rỉ `*pgx.Tx` ra tầng API hay Domain.
