package auth

const Schema = `


CREATE TABLE IF NOT EXISTS
accounts (
	account_id SERIAL PRIMARY KEY,
	provider TEXT NOT NULL,
	name TEXT NOT NULL,
	profile_url TEXT NOT NULL UNIQUE,
	created TIMESTAMPTZ NOT NULL
);

`
