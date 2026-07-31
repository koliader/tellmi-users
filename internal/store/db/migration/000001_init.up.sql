CREATE TABLE users (
  "id" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "role" varchar NOT NULL DEFAULT 'USER',
  "password" varchar NOT NULL,
  "username" varchar UNIQUE NOT NULL,
  "is_blocked" bool NOT NULL DEFAULT false
);
