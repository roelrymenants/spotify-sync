package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/roelrymenants/spotify"
)

const (
	DefaultWwwRoot  = "/home/nitrous/www/"
	clientId        = "b57e5e2127014c8d9e45da5e507936bb"
	clientSecret    = "8b980072b2cf4f52ba05a75b7391ba46"
	DefaultAuthFile = "/home/nitrous/spotify-sync.auth-"
)

type Page struct {
	Login         string
	Authenticated bool
}

var store = sessions.NewCookieStore([]byte("spotify-sync-secret"))

var connections map[string]*spotify.SpotifyConnection

type connectionHandler struct {
	h http.Handler
}

func (c connectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "spotify-sync")

	login, loginOk := session.Values["login"].(string)

	context.Set(r, "login", login)

	connection, connectionOk := connections[login]
	if !connectionOk && loginOk {
		connection = spotify.NewConnection(clientId, clientSecret)
		connection.Load(authFile + login)
	}

	context.Set(r, "connection", connection)

	c.h.ServeHTTP(w, r)
}

var authFile string

func main() {
	var wwwRoot = *flag.String("wwwRoot", DefaultWwwRoot, "Root directory from which to serve")
	authFile = *flag.String("authFile", DefaultAuthFile, "File to load/save auth data from/to")
	flag.Parse()

	http.HandleFunc("/api/trackList", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*spotify.SpotifyConnection)
		trackList := connection.GetSpotifyTrackList()

		encoder := json.NewEncoder(w)
		encoder.Encode(trackList)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*spotify.SpotifyConnection)
		authCode := r.FormValue("code")
		login := context.Get(r, "login").(string)

		err := connection.DoAuth(authCode, "http://go-102859.nitrousapp.com:8080/callback")

		if err != nil {
			log.Fatal(err)
		}

		err = connection.Save(authFile + login)

		if err != nil {
			log.Fatal(err)
		}

		http.Redirect(w, r, "/", 301)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*spotify.SpotifyConnection)

		var page Page

		page.Login = context.Get(r, "login").(string)

		if connection != nil && connection.Authentication != nil {
			log.Printf("%v %v", connection, connection.Authentication)
			page.Authenticated = true
		}

		t, _ := template.ParseFiles(wwwRoot + "index.html")
		t.Execute(w, page)
	})

	log.Fatal(http.ListenAndServe(":8080", context.ClearHandler(connectionHandler{http.DefaultServeMux})))
}
