-- name: FollowUser :exec
INSERT INTO followers (
    follower_id,
    followed_user_id
)
VALUES ($1, $2);

-- name: UnfollowUser :exec
DELETE FROM followers
WHERE follower_id = $1
  AND followed_user_id = $2;
