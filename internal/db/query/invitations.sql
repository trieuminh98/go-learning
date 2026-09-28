-- name: CreateInvitation :exec
INSERT INTO user_invitations (
    token_hash,
    user_id,
    expiry
)
VALUES ($1, $2, $3);

-- name: DeleteInvitationByUserID :exec
DELETE FROM user_invitations
WHERE user_id = $1;

-- name: GetInvitationByTokenHash :one
SELECT token_hash, user_id, expiry
FROM user_invitations
WHERE token_hash = $1;
