package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"

	"github.com/GeertJohan/go.rice"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/zmb3/spotify"
)

const (
	DefaultWwwRoot  = "www"
	clientId        = "b57e5e2127014c8d9e45da5e507936bb"
	clientSecret    = "8b980072b2cf4f52ba05a75b7391ba46"
	DefaultAuthFile = "/home/nitrous/spotify-sync.auth-"
	RedirectUrl     = "http://go-102859.nitrousapp.com:8080/callback"
	state           = "123" //Default state
)

type Page struct {
	Login         string
	Authenticated bool
}

var store = sessions.NewCookieStore([]byte("spotify-sync-secret"))

var connections map[string]*spotify.Client

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

	context.Set(r, "login", login)

	connection, connectionOk := connections[login]
	log.Print(r.URL.Path)
	if !connectionOk && loginOk && r.URL.Path == "/callback" {
		token, err := c.Authenticator.Token(state, r)

		if err != nil {
			log.Fatal(err)
		}

		tConn := c.Authenticator.NewClient(token)
		connection = &tConn
		connections[login] = connection

		http.Redirect(w, r, "/", 301)
	}

	context.Set(r, "connection", connection)

	c.h.ServeHTTP(w, r)
}

var authFile string

func main() {
	var wwwRoot = *flag.String("wwwRoot", DefaultWwwRoot, "Root directory from which to serve")
	authFile = *flag.String("authFile", DefaultAuthFile, "File to load/save auth data from/to")
	flag.Parse()

	wwwBox := rice.MustFindBox(wwwRoot)

	connections = make(map[string]*spotify.Client)

	http.HandleFunc("/api/trackList", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*spotify.Client)
		trackList, err := connection.CurrentUsersTracks()

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		connection := context.Get(r, "connection").(*spotify.Client)

		var page Page

		page.Login = context.Get(r, "login").(string)

		if connection != nil && connection != nil {
			log.Printf("%v %v", connection)
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
