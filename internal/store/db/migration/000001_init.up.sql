CREATE TABLE "Users" (
  "id" integer PRIMARY KEY,
  "password" varchar NOT NULL,
  "username" varchar UNIQUE NOT NULL,
  "is_blocked" bool DEFAULT false
);

CREATE TABLE "Boards" (
  "id" integer PRIMARY KEY,
  "user_id" integer NOT NULL
);

CREATE TABLE "Feedbacks" (
  "id" integer PRIMARY KEY,
  "user_id" integer NOT NULL,
  "board_id" integer NOT NULL
);

ALTER TABLE "Boards" ADD FOREIGN KEY ("user_id") REFERENCES "Users" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "Feedbacks" ADD FOREIGN KEY ("user_id") REFERENCES "Users" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "Feedbacks" ADD FOREIGN KEY ("board_id") REFERENCES "Boards" ("id") DEFERRABLE INITIALLY IMMEDIATE;
