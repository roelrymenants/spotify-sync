package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"math"
	"net/http"

	"github.com/GeertJohan/go.rice"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/roelrymenants/spotify"
)

const (
	DefaultWwwRoot = "www"
	clientId       = "b57e5e2127014c8d9e45da5e507936bb"
	clientSecret   = "8b980072b2cf4f52ba05a75b7391ba46"
	RedirectUrl    = "http://go-102859.nitrousapp.com:8080/callback"
	state          = "123" //Default state
)

type SpotifyClient struct {
	spotify.Client
}

type Page struct {
	Login         string
	Authenticated bool
	Imported      bool
}

var store = sessions.NewCookieStore([]byte("spotify-sync-secret"))

var connections map[string]*SpotifyClient

type connectionHandler struct {
	h             http.Handler
	Authenticator *spotify.Authenticator
}

func newConnectionHandler() *connectionHandler {
	auth := spotify.NewAuthenticator(RedirectUrl, spotify.ScopeUserLibraryRead)
	auth.SetAuthInfo(clientId, clientSecret)

	return &connectionHandler{
		h:             http.DefaultServeMux,
		Authenticator: &auth,
	}
}

func (c connectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "spotify-sync")

	login, loginOk := session.Values["login"].(string)

	imported, _ := session.Values["imported"].(bool)

	context.Set(r, "login", login)
	context.Set(r, "imported", imported)

	connection, connectionOk := connections[login]
	log.Print(r.URL.Path)
	if !connectionOk && loginOk && r.URL.Path == "/callback" {
		token, err := c.Authenticator.Token(state, r)

		if err != nil {
			log.Fatal(err)
		}

		tConn := SpotifyClient{c.Authenticator.NewClient(token)}
		connection = &tConn
		connections[login] = connection

		http.Redirect(w, r, "/", 301)
	}

	context.Set(r, "connection", connection)

	c.h.ServeHTTP(w, r)
}

func main() {
	var wwwRoot = *flag.String("wwwRoot", DefaultWwwRoot, "Root directory from which to serve")
	flag.Parse()

	wwwBox := rice.MustFindBox(wwwRoot)

	connections = make(map[string]*SpotifyClient)

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

		delete(session.Values, "login")

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		//Hack dummy handlefunc
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
				err = connection.AddTracksToLibrary(trackIDs...)
				if err != nil {
					log.Fatal(err)
				}
			}

			trackIDs[pageIndex] = track.ID
		}

		session.Values["imported"] = true

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*SpotifyClient)

		var page Page

		page.Login = context.Get(r, "login").(string)
		page.Imported = context.Get(r, "imported").(bool)

		if connection != nil && connection != nil {
			page.Authenticated = true
		}

		template, err := loadTemplate("index", wwwBox)

		if err != nil {
			log.Fatal(err)
		}

		template.Execute(w, page)
	})

	log.Fatal(http.ListenAndServe(":8080", context.ClearHandler(newConnectionHandler())))
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

func (c *SpotifyClient) getLibraryTrackList() (*spotify.SavedTrackPage, error) {
	return c.getLibraryTrackListRec(0, nil)
}

func (c *SpotifyClient) getLibraryTrackListRec(offset int, trackList *spotify.SavedTrackPage) (*spotify.SavedTrackPage, error) {
	limit := 50

	opt := spotify.Options{
		Limit:  &limit,
		Offset: &offset,
	}

	currentTrackList, err := c.CurrentUsersTracksOpt(&opt)

	if err != nil {
		return nil, err
	}

	if trackList == nil {
		trackList = currentTrackList
	} else {
		trackList.Tracks = append(trackList.Tracks, currentTrackList.Tracks...)
	}

	if currentTrackList.Next != "" {
		_, err = c.getLibraryTrackListRec(offset+limit, trackList)
		if err != nil {
			return nil, err
		}
	}

	return trackList, nil
}
