package stream

const Schema = `

CREATE TABLE IF NOT EXISTS
feeds (
	feed_id SERIAL PRIMARY KEY,
	url TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	favicon_url TEXT NOT NULL DEFAULT '',
	updated TIMESTAMPTZ NOT NULL,
	owned_by INTEGER NOT NULL -- references account, but if 0, not owned by anyone
);


---

CREATE TABLE IF NOT EXISTS
subscriptions (
	subscription_id SERIAL PRIMARY KEY,
	account_id INTEGER NOT NULL, --  REFERENCES accounts(account_id)
	feed_id INTEGER NOT NULL REFERENCES feeds(feed_id),
	created TIMESTAMPTZ NOT NULL,

	UNIQUE (account_id, feed_id)
);

---

CREATE TABLE IF NOT EXISTS
entries (
	entry_id SERIAL PRIMARY KEY,
	feed_id INTEGER REFERENCES feeds(feed_id), -- points to user bookmark feed when bookmark
	title TEXT NOT NULL,
	url TEXT NOT NULL,
	created TIMESTAMPTZ NOT NULL,
	published TIMESTAMPTZ NOT NULL, -- same as created for bookmarks
	word_count INTEGER NOT NULL default 0,

	UNIQUE(feed_id, url)
);

---

CREATE OR REPLACE FUNCTION
subscribe(account_id integer, feed_url text, title text, now timestamptz) RETURNS INTEGER AS $$
DECLARE
	fid INTEGER;
BEGIN
	SELECT feed_id INTO fid FROM feeds WHERE url = feed_url LIMIT 1;
	IF NOT FOUND THEN
		-- create new feeds with update time == 0, so that all entries
		-- fetched it first run will be accepted
		INSERT INTO feeds (url, title, updated, owned_by)
			VALUES (feed_url, title, cast('epoch' AS timestamptz), 0)
			RETURNING feed_id
			INTO fid;
	END IF;

	INSERT INTO subscriptions (account_id, feed_id, created)
		VALUES (account_id, fid, now)
		ON CONFLICT DO NOTHING;

	RETURN fid;
END;
$$ LANGUAGE plpgsql;


`
