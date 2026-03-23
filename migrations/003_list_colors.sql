CREATE TABLE IF NOT EXISTS list_colors (
    user_email TEXT NOT NULL,
    list_id TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_email, list_id)
);
