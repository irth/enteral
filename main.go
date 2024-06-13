package main

import (
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/context"
)

type App struct {
	cache Cache
}

func (a App) IsShort(ctx context.Context, id string) (bool, error) {
	cached, err := a.cache.Get(ctx, "is-short", id)
	if err != nil && err != KeyNotFound {
		log.Printf("IsShort: cache get error: %s", err)
	} else if err == nil {
		return cached == "1", nil
	}

	shortURL := fmt.Sprintf("https://www.youtube.com/shorts/%s", id)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// TODO: context
	res, err := client.Head(shortURL)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 || res.StatusCode <= 199 {
		return false, fmt.Errorf("http error %d", res.StatusCode)
	}

	isShort := res.StatusCode <= 299

	str := "0"
	if isShort {
		str = "1"
	}
	err = a.cache.Set(ctx, "is-short", id, time.Hour*72, str)
	if err != nil {
		log.Printf("IsShort: cache set error: %s", err)
	}

	return isShort, nil
}

func GetId(videoUrl string) (string, error) {
	u, err := url.Parse(videoUrl)
	if err != nil {
		return "", err
	}

	v := u.Query().Get("v")
	if v == "" {
		return "", fmt.Errorf("couldn't extract video id from url")
	}
	return v, nil
}

var ErrNotFound = errors.New("not found")

func (a App) FilterFeed(ctx context.Context, id string, url string) (*Feed, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		if err, ok := err.(gofeed.HTTPError); ok && err.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	newFeed := Feed{
		Xmlns:      "http://www.w3.org/2005/Atom",
		XmlnsMedia: "http://search.yahoo.com/mrss/",
		XmlnsYt:    "http://www.youtube.com/xml/schemas/2015",
		Id:         fmt.Sprintf("enteral:channel:%s", id),
		Title:      feed.Title,
		Links:      []Link{{Rel: "alternate", Href: feed.Link}},
		Published:  feed.Published,
	}

	var author *Author = nil

	if len(feed.Authors) > 0 {
		author = &Author{
			Name: feed.Authors[0].Name,
			Uri:  feed.Link,
		}
		newFeed.Author = author
	}

	for _, item := range feed.Items {
		id, err := GetId(item.Link)
		if err != nil {
			log.Printf("couldn't get ID for %s", item.Link)
			continue
		}
		short, err := a.IsShort(ctx, id)
		if err != nil {
			log.Printf("couldn't determine short status for %s", item.Link)
			continue
		}

		if short {
			continue
		}

		newItem := Entry{
			Title:     item.Title,
			ID:        item.GUID,
			Author:    author,
			Published: item.Published,
			Updated:   item.Updated,
		}

		newFeed.Entries = append(newFeed.Entries, newItem)
	}

	return &newFeed, nil
}

//go:embed index.html
var index []byte

func (a App) Root(w http.ResponseWriter, r *http.Request) {
	w.Write(index)
}

func (a App) Feed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Meow", "mrrp")

	chId := r.URL.Query().Get("channel_id")
	if chId == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, "no channel_id")
		return
	}
	if len(chId) > 64 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, "weirdly long channel_id")
		return
	}

	cached, err := a.cache.Get(r.Context(), "filtered-feed", chId)
	if err != nil && err != KeyNotFound {
		log.Printf("Feed: cache get error: %s", err)
	} else if err == nil {
		if cached == "404" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "feed not found")
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=UTF-8")
		fmt.Fprint(w, cached)
		return
	}

	qs := url.Values{}
	qs.Set("channel_id", chId)

	feedUrl := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?%s", qs.Encode())
	newFeed, err := a.FilterFeed(r.Context(), chId, feedUrl)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "feed not found")
			err = a.cache.Set(r.Context(), "filtered-feed", chId, time.Hour*72, "404")
			if err != nil {
				log.Printf("Feed: cache set error (404): %s", err)
			}
			return
		}
		log.Printf("while processing %s: %s", feedUrl, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error")
		return
	}

	filteredFeedB, err := xml.MarshalIndent(newFeed, "", "    ")
	if err != nil {
		log.Printf("while generating atom for %s: %s", feedUrl, err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error")
		return
	}
	filteredFeed := xml.Header + string(filteredFeedB)

	w.Header().Set("Content-Type", "text/xml; charset=UTF-8")
	fmt.Fprint(w, filteredFeed)

	err = a.cache.Set(r.Context(), "filtered-feed", chId, time.Minute*15, filteredFeed)
	if err != nil {
		log.Printf("Feed: cache set error: %s", err)
	}
}

func (a App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/", a.Root)
	r.Get("/feeds/videos.xml", a.Feed)
	return r
}

func main() {
	noCache := os.Getenv("NO_CACHE") == "1"

	redisUrl := "redis://127.0.0.1:6379"
	if redisEnv := os.Getenv("REDIS_URL"); redisEnv != "" {
		redisUrl = redisEnv
	}

	listenAddr := ":5000"
	if listenEnv := os.Getenv("LISTEN_ADDR"); listenEnv != "" {
		listenAddr = listenEnv
	}

	var cache Cache = DummyCache{}

	if !noCache {
		vc, err := NewValkeyIsShortCache(redisUrl)
		if err != nil {
			log.Fatal(err)
		}
		cache = vc
	}
	app := App{cache: cache}

	log.Printf("listening on %s", listenAddr)
	err := http.ListenAndServe(listenAddr, app.Router())
	if err != nil {
		log.Fatal(err)
	}
}
