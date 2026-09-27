CREATE TABLE IF NOT EXISTS followers (
    follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followed_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followed_user_id)
);

CREATE INDEX IF NOT EXISTS idx_followers_followed_user_id ON followers(followed_user_id);
