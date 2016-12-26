package stream

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/husio/feedstream/auth"
	"github.com/husio/feedstream/ui"
	"github.com/husio/web"
)

func EntriesHandler(
	manager Manager,
	authSrv auth.AuthService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := authSrv.CurrentUser(r.Context(), r)
		switch err {
		case nil:
			// all good
		case auth.ErrNotAuthenticated:
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		default:
			log.Printf("cannot get current user: %s", err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		var (
			entries []*Entry
			feed    *Feed
		)
		feedID, err := strconv.ParseInt(r.URL.Query().Get("feed"), 10, 64)
		if feedID > 0 {
			feed, err = manager.Feed(r.Context(), feedID)
			if err != nil {
				log.Printf("cannot fetch feed %d: %s", feedID, err)
			}
			entries, err = manager.FeedEntries(r.Context(), user.AccountID, feedID, time.Now())
		} else {
			entries, err = manager.Entries(r.Context(), user.AccountID, time.Now())
		}
		if err != nil {
			log.Printf("cannot list: %s", err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		content := struct {
			Feed    *Feed
			Entries []*Entry
		}{
			Feed:    feed,
			Entries: entries,
		}
		tmpl.Render(w, "entrylist.tmpl", content, http.StatusOK)
	}
}

func SubscriptionHandler(
	manager Manager,
	bookmarklet BookmarkletRenderer,
	authSrv auth.AuthService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := authSrv.CurrentUser(r.Context(), r)
		switch err {
		case nil:
			// all good
		case auth.ErrNotAuthenticated:
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		default:
			log.Printf("cannot get current user: %s", err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		url := strings.TrimSpace(r.FormValue("url"))

		if r.Method == "GET" || url == "" {
			subs, err := manager.Subscriptions(r.Context(), user.AccountID)
			if err != nil {
				log.Printf("cannot list subscriptions: %s", err)
			}

			bookmarkletAttr, err := bookmarklet.RenderAttr("xxx") // TODO
			if err != nil {
				log.Printf("cannot render bookmarklet attribute: %s", err)
			}

			content := struct {
				Subscriptions   []*Subscription
				BookmarkletHref template.HTMLAttr
			}{
				Subscriptions:   subs,
				BookmarkletHref: bookmarkletAttr,
			}
			tmpl.Render(w, "subscribe.tmpl", content, http.StatusOK)
			return
		}

		feedID, err := manager.Subscribe(r.Context(), user.AccountID, url)
		if err != nil {
			log.Printf("cannot subscribe to %q: %s", url, err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		if err := manager.Update(r.Context(), feedID); err != nil {
			log.Printf("cannot update subscription %q: %s", url, err)
		}

		http.Redirect(w, r, "/subscriptions", http.StatusFound)
	}
}

func UpdateOutdatedHandler(
	manager Manager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := manager.OutdatedFeeds(r.Context(), time.Now().Add(-2*time.Hour))
		if err != nil {
			log.Printf("cannot fetch outdated feeds: %s", err)
			web.StdJSONResp(w, http.StatusInternalServerError)
			return
		}

		for _, id := range ids {
			go func(id int64) {
				log.Printf("updating feed: %d", id)
				if err := manager.Update(r.Context(), id); err != nil {
					log.Printf("cannot update feed %d: %s", id, err)
				}
			}(id)
		}
		web.JSONResp(w, len(ids), http.StatusAccepted)
	}
}

func RemoveSubscriptionHandler(
	manager Manager,
	authSrv auth.AuthService,
	tmpl ui.Renderer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := authSrv.CurrentUser(r.Context(), r)
		switch err {
		case nil:
			// all good
		case auth.ErrNotAuthenticated:
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		default:
			log.Printf("cannot get current user: %s", err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		subID, err := strconv.ParseInt(web.PathArg(r, 0), 10, 64)
		if err != nil {
			tmpl.RenderStd(w, http.StatusBadRequest)
			return
		}

		if err := manager.Unsubscribe(r.Context(), subID, user.AccountID); err != nil {
			log.Printf("cannot unsubscribe %d: %s", subID, err)
			tmpl.RenderStd(w, http.StatusInternalServerError)
			return
		}

		next := r.Referer()
		if next == "" {
			next = "/subscriptions"
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	}
}

func BookmarkHandler(
	manager Manager,
	authSrv auth.AuthService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorize")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusOK)
			return
		}

		user, err := authSrv.CurrentUser(r.Context(), r)
		switch err {
		case nil:
			// all good
		case auth.ErrNotAuthenticated:
			web.JSONRedirect(w, "/login", http.StatusTemporaryRedirect)
			return
		default:
			log.Printf("cannot get current user: %s", err)
			web.StdJSONResp(w, http.StatusInternalServerError)
			return
		}

		// TODO: use bookmarklet key as authentication method
		var input struct {
			Title string `json:"title"`
			Url   string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			web.JSONErr(w, "invalid input json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if input.Url == "" {
			web.JSONErr(w, `"url" is required`, http.StatusBadRequest)
			return
		}

		if err := manager.Bookmark(r.Context(), user.AccountID, input.Url, input.Title); err != nil {
			log.Printf("cannot create bookmark: %s", err)
			web.JSONErr(w, "cannot create bookmark", http.StatusInternalServerError)
			return
		}
		web.StdJSONResp(w, http.StatusCreated)
	}
}

func BookmarkletHandler() http.HandlerFunc {
	const content = `<!doctype html>
<script>
window.addEventListener("message", function (ev) {
    var req = new window.XMLHttpRequest()
    req.open('POST', "/bookmarks", true)
    req.setRequestHeader('Content-Type', 'application/json')
    req.send(JSON.stringify(ev.data))
}, false)
</script>`
	return func(w http.ResponseWriter, r *http.Request) {
		//w.Header().Set("Cache-Control", "max-age=604800")
		io.WriteString(w, content)
	}
}
