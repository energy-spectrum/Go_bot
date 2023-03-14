CREATE TABLE "requests" (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    cryptocurrency TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);