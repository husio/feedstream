package stream

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/husio/feedstream/pg"
)

type Manager interface {
	Entries(ctx context.Context, accountID int64, publishedLte time.Time) ([]*Entry, error)
	FeedEntries(ctx context.Context, accountID, feedID int64, publishedLte time.Time) ([]*Entry, error)
	Feed(ctx context.Context, feedID int64) (*Feed, error)
	Subscriptions(ctx context.Context, accountID int64) ([]*Subscription, error)
	Subscribe(ctx context.Context, accountID int64, feedUrl string) (int64, error)
	Unsubscribe(ctx context.Context, subscriptionID, accountID int64) error
	Update(ctx context.Context, feedID int64) error
	OutdatedFeeds(ctx context.Context, updatedLte time.Time) ([]int64, error)
	Bookmark(ctx context.Context, accountID int64, url, title string) error
}

type Entry struct {
	EntryID        int64  `db:"entry_id"`
	FeedID         int64  `db:"feed_id"`
	FeedTitle      string `db:"feed_title"`
	FeedFaviconURL string `db:"feed_favicon_url"`
	FeedOwnedBy    int64  `db:"feed_owned_by"`
	WordCount      int    `db:"word_count"`
	CanDelete      bool   `db:"can_delete"`
	Title          string
	URL            string
	Published      time.Time
	Created        time.Time
}

func (e *Entry) URLHost() string {
	u, err := url.Parse(e.URL)
	if err != nil {
		return ""
	}
	return u.Host
}

func (e *Entry) ReadingTime() string {
	if e.WordCount == 0 {
		return ""
	}
	const wpm = 200
	min := e.WordCount / wpm
	switch {
	case min == 0:
		return "less than 1 minute"
	case min > 20:
		return "more than 20 minutes"
	case min == 1:
		return "1 minute"
	default:
		return fmt.Sprintf("%d minutes", min)
	}
}

type Subscription struct {
	SubscriptionID int64  `db:"subscription_id"`
	FeedID         int64  `db:"feed_id"`
	FeedOwnedBy    int64  `db:"feed_owned_by"`
	FeedFaviconURL string `db:"feed_favicon_url"`
	AccountID      int64  `db:"account_id"`
	Title          string
	URL            string
	Created        time.Time
	Updated        time.Time
}

func (s *Subscription) Host() string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}
	return u.Host
}

type Feed struct {
	FeedID     int64 `db:"feed_id"`
	Title      string
	FaviconURL string `db:"favicon_url"`
	URL        string
	Updated    time.Time
	OwnedBy    int64 `db:"owned_by"`
}

type manager struct {
	db        pg.Database
	rp        *redis.Pool
	newspaper NewspaperService
}

var _ Manager = (*manager)(nil)

func NewManager(db pg.Database, rp *redis.Pool, n NewspaperService) Manager {
	return &manager{
		db:        db,
		rp:        rp,
		newspaper: n,
	}
}

func (m *manager) Entries(ctx context.Context, accountID int64, publishedLte time.Time) ([]*Entry, error) {
	var entries []*Entry
	err := m.db.Select(&entries, `
		SELECT
			e.entry_id,
			e.feed_id,
			e.title,
			e.url,
			e.published,
			e.created,
			e.word_count,
			f.owned_by AS feed_owned_by,
			f.title AS feed_title,
			f.favicon_url AS feed_favicon_url
		FROM
			entries e
			INNER JOIN feeds f ON e.feed_id = f.feed_id
			INNER JOIN subscriptions s ON s.feed_id = f.feed_id
		WHERE
			s.account_id = $1
			AND e.published <= $2
		ORDER BY
			e.published DESC
		LIMIT 100
	`, accountID, publishedLte)
	return entries, err
}

func (m *manager) FeedEntries(ctx context.Context, accountID, feedID int64, publishedLte time.Time) ([]*Entry, error) {
	var entries []*Entry
	err := m.db.Select(&entries, `
		SELECT
			e.entry_id,
			e.feed_id,
			e.title,
			e.url,
			e.word_count,
			e.published,
			e.created,
			f.owned_by AS feed_owned_by,
			f.title AS feed_title,
			f.favicon_url AS feed_favicon_url
		FROM
			entries e
			INNER JOIN feeds f ON e.feed_id = f.feed_id
			INNER JOIN subscriptions s ON s.feed_id = f.feed_id
		WHERE
			s.account_id = $1
			AND e.published <= $2
			AND e.feed_id = $3
		ORDER BY
			e.published DESC
		LIMIT 200
	`, accountID, publishedLte, feedID)
	return entries, err
}

func (m *manager) Subscriptions(ctx context.Context, accountID int64) ([]*Subscription, error) {
	var subs []*Subscription
	err := m.db.Select(&subs, `
		SELECT
			s.subscription_id,
			s.feed_id,
			s.account_id,
			f.title,
			f.url,
			s.created,
			f.updated,
			f.owned_by AS feed_owned_by,
			f.favicon_url AS feed_favicon_url
		FROM
			subscriptions s
			INNER JOIN feeds f ON s.feed_id = f.feed_id
		WHERE
			s.account_id = $1
		ORDER BY
			f.title ASC
		LIMIT 1000
	`, accountID)
	return subs, err
}

func (m *manager) Subscribe(ctx context.Context, accountID int64, feedUrl string) (int64, error) {
	// before adding to database, test if given url can be trusted and
	// points to feed
	if _, err := fetchFeed(ctx, feedUrl); err != nil {
		return 0, fmt.Errorf("invalid feed: %s", err)
	}

	var (
		feedID int64
		title  string
	)
	if u, err := url.Parse(feedUrl); err == nil {
		title = u.Host
	} else {
		title = feedUrl
	}
	err := m.db.Get(&feedID, `
		SELECT subscribe($1, $2, $3, $4)
	`, accountID, feedUrl, title, time.Now())
	return feedID, err
}

func (m *manager) Feed(ctx context.Context, feedID int64) (*Feed, error) {
	var f Feed
	err := m.db.Get(&f, `
		SELECT * FROM feeds WHERE feed_id = $1
		LIMIT 1
	`, feedID)
	return &f, err
}

func (m *manager) Update(ctx context.Context, feedID int64) error {
	rc := m.rp.Get()
	defer rc.Close()

	// prevent from updates happening too often using lock with expiration
	lockKey := fmt.Sprintf("manager.Update:%d", feedID)
	switch _, err := rc.Do("SET", lockKey, "1", "EX", 30, "NX"); err {
	case redis.ErrNil:
		return nil // lock not released
	case nil:
		// all good
	default:
		return fmt.Errorf("cannot lock update: %s", err)
	}

	tx, err := m.db.Beginx()
	if err != nil {
		return fmt.Errorf("cannot start transaction: %s", err)
	}
	defer tx.Rollback()

	var feed struct {
		FeedID     int64  `db:"feed_id"`
		FaviconURL string `db:"favicon_url"`
		Title      string
		URL        string
		Updated    time.Time
	}

	err = tx.Get(&feed, `
		SELECT
			feed_id,
			url,
			title,
			favicon_url,
			updated
		FROM feeds
		WHERE feed_id = $1
		LIMIT 1
	`, feedID)
	if err != nil {
		return fmt.Errorf("cannot fetch feed: %s", err)
	}

	fi, err := fetchFeed(ctx, feed.URL)
	if err != nil {
		return fmt.Errorf("cannot fetch: %s", err)
	}

	if fi.Title() != "" {
		feed.Title = fi.Title()
	}
	if feed.FaviconURL == "" {
		if fav, err := fi.FaviconURL(ctx); err != nil {
			log.Printf("cannot fetch favicon url: %s", err)
		} else {
			feed.FaviconURL = fav
		}
	}

	now := time.Now()
	_, err = tx.Exec(`
		UPDATE feeds
		SET title = $1, updated = $2, favicon_url = $3
		WHERE feed_id = $4
	`, feed.Title, now, feed.FaviconURL, feedID)
	if err != nil {
		return fmt.Errorf("cannot update feed: %s", err)
	}

	for _, entry := range fi.Entries() {
		if entry.Published.Before(feed.Updated) {
			continue
		}

		var wordCnt int
		meta, err := m.newspaper.Article(ctx, entry.URL)
		if err != nil {
			log.Printf("cannot fetch article %q: %s", entry.URL, err)
		} else {
			wordCnt = len(strings.Fields(meta.Text))
		}

		_, err = tx.Exec(`
			INSERT INTO entries (feed_id, title, url, published, created, word_count)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING
		`, feed.FeedID, entry.Title, entry.URL, entry.Published, now, wordCnt)
		if err != nil {
			return fmt.Errorf("cannot insert entry: %s", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot commit transaction: %s", err)
	}
	return nil
}

func (m *manager) OutdatedFeeds(ctx context.Context, updatedLte time.Time) ([]int64, error) {
	var ids []int64
	err := m.db.Select(&ids, `
		SELECT feed_id
		FROM feeds
		WHERE updated <= $1 AND autorefresh = true
		LIMIT 500
	`, updatedLte)
	return ids, err
}

func (m *manager) Unsubscribe(ctx context.Context, subID, accID int64) error {
	_, err := m.db.Exec(`
		DELETE FROM subscriptions
		WHERE account_id = $1 AND subscription_id = $2
	`, accID, subID)
	return err
}

func (m *manager) Bookmark(ctx context.Context, accountID int64, url, title string) error {
	if title == "" {
		title = url
	}
	tx, err := m.db.Beginx()
	if err != nil {
		return fmt.Errorf("cannot start transaction: %s", err)
	}
	defer tx.Rollback()

	feedUrl := fmt.Sprintf("/?feed=%d", accountID)
	now := time.Now()

	_, err = tx.Exec(`
		INSERT INTO feeds (url, updated, owned_by, title, favicon_url)
		VALUES ($1, $2, $3, 'Bookmarks', '/static/bookmark.png')
		ON CONFLICT DO NOTHING
	`, feedUrl, now, accountID)
	if err != nil {
		return fmt.Errorf("cannot ensure bookmark feed exists: %s", err)
	}
	_, err = tx.Exec(`
		INSERT INTO subscriptions (feed_id, account_id, created)
		VALUES (
			(SELECT feed_id FROM feeds WHERE owned_by = $1 LIMIT 1),
			$1, $2
		)
		ON CONFLICT DO NOTHING

	`, accountID, now)
	if err != nil {
		return fmt.Errorf("cannot ensure bookmark subscription exists: %s", err)
	}
	_, err = tx.Exec(`
		INSERT INTO entries (feed_id, title, url, created, published, word_count)
		VALUES (
			(SELECT feed_id FROM feeds WHERE owned_by = $1 LIMIT 1),
			$2, $3, $4, $4, $5)
		ON CONFLICT (feed_id, url) DO UPDATE SET
			published = $4,
			title = $2,
			word_count = $5
	`, accountID, title, url, time.Now(), 0) // TODO
	if err != nil {
		return fmt.Errorf("cannot insert bookmark: %s", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot commit transaction: %s", err)
	}
	return nil
}
