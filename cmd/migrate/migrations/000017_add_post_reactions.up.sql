CREATE TABLE post_reactions (
    user_id BIGINT NOT NULL,
    post_id BIGINT NOT NULL,
    reaction_type VARCHAR(50) NOT NULL DEFAULT 'like',
    PRIMARY KEY (user_id, post_id),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE
);
