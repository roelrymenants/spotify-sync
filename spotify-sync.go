package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/GeertJohan/go.rice"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/zmb3/spotify"
)

const (
	DefaultWwwRoot = "www"
	CallbackPath   = "/callback"
	state          = "123" //Default state
	DefaultHost    = "localhost"
	DefaultScheme  = "http"
	DefaultPort    = ":8080"
)

type Page struct {
	Login         string
	Authenticated bool
	Flashes       []interface{}
	AuthUrl       string
}

var store = sessions.NewCookieStore([]byte("spotify-sync-secret"))

var connections map[string]*SpotifyClient

func main() {
	var wwwRoot = flag.String("wwwRoot", DefaultWwwRoot, "Root directory from which to serve")
	var host = flag.String("host", DefaultHost, "Domain on which the app is served (e.g. 'localhost')")
	var port = flag.String("port", DefaultPort, "Binding on which to listen (e.g. ':8080')")
	flag.Parse()

	clientId := os.Getenv("SPOTIFY_SYNC_ID")

	if clientId == "" {
		log.Fatal("Must provide spotify client id using environment variable 'SPOTIFY_SYNC_ID'")
	}

	clientSecret := os.Getenv("SPOTIFY_SYNC_SECRET")

	if clientSecret == "" {
		log.Fatal("Must provide spotify client secret using environment variable 'SPOTIFY_SYNC_SECRET'")
	}

	wwwBox := rice.MustFindBox(*wwwRoot)

	http.HandleFunc("/api/trackList", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*SpotifyClient)

		download := r.FormValue("export")

		if download != "" {
			w.Header().Set("content-type", "application/octet-stream")
		}

		trackList, err := connection.getLibraryTrackList()

		if err != nil {
			log.Fatal(err)
		}

		encoder := json.NewEncoder(w)
		encoder.Encode(trackList)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "spotify-sync")

		session.Values["login"] = r.FormValue("login")

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "spotify-sync")

		//Clear entire session
		session.Options = &sessions.Options{MaxAge: -1}

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "spotify-sync")
		connection := context.Get(r, "connection").(*SpotifyClient)

		if connection == nil {
			log.Fatal("got no connection")
		}

		r.ParseMultipartForm(32 << 20)
		file, _, err := r.FormFile("import")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)

		var trackList spotify.SavedTrackPage

		err = decoder.Decode(&trackList)

		if err != nil {
			log.Fatal(err)
		}

		var trackIDs []spotify.ID = make([]spotify.ID, 50)

		for i, track := range trackList.Tracks {
			pageIndex := int(math.Mod(float64(i), float64(50)))

			if i != 0 && pageIndex == 0 {
				log.Print(trackIDs)
				time.Sleep(1 * time.Second)
				err = connection.AddTracksToLibrary(trackIDs...)
				if err != nil {
					log.Fatal(err)
				}
			}

			trackIDs[pageIndex] = track.ID
		}

		session.AddFlash("Import succeeded")

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		page := context.Get(r, "page").(Page)

		template, err := loadTemplate("index", wwwBox)

		if err != nil {
			log.Fatal(err)
		}

		template.Execute(w, page)
	})

	redirectUrl := url.URL{
		Scheme: "http",
		Host:   *host + *port,
		Path:   CallbackPath,
	}

	connectionHandler := newConnectionHandler(redirectUrl, clientId, clientSecret)

	log.Fatal(http.ListenAndServe(*port, context.ClearHandler(connectionHandler)))
}

func loadTemplate(name string, wwwBox *rice.Box) (t *template.Template, err error) {
	templateString, err := wwwBox.String("index.tmpl")

	if err != nil {
		return nil, err
	}

	t, err = template.New("index").Parse(templateString)

	if err != nil {
		return nil, err
	}

	return
}
