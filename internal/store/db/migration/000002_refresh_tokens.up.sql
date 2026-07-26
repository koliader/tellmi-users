CREATE TABLE "refresh_tokens" (
  "token" varchar PRIMARY KEY,
  "username" varchar NOT NULL,
  "expires_at" timestamptz NOT NULL
);
