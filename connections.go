package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/context"
	"github.com/zmb3/spotify"
)

type connectionHandler struct {
	h             http.Handler
	clientId      string
	clientSecret  string
	Authenticator *spotify.Authenticator
	connections   map[string]*SpotifyClient
}

func newConnectionHandler(redirectUrl url.URL, clientId string, clientSecret string) *connectionHandler {
	auth := spotify.NewAuthenticator(redirectUrl.String(), spotify.ScopeUserLibraryRead, spotify.ScopeUserLibraryModify)
	auth.SetAuthInfo(clientId, clientSecret)

	return &connectionHandler{
		h:             http.DefaultServeMux,
		clientId:      clientId,
		clientSecret:  clientSecret,
		Authenticator: &auth,
		connections:   make(map[string]*SpotifyClient),
	}
}

func (c connectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "spotify-sync")

	state, stateOk := session.Values["state"].(string)

	if !stateOk {
		n := 5
		b := make([]byte, n)
		_, err := rand.Read(b)
		if err != nil {
			log.Fatal(err)
		}

		state := string(b[:n])

		session.Values["state"] = state

		session.Save(r, w)
	}

	login, loginOk := session.Values["login"].(string)

	flashes := session.Flashes()

	connection, connectionOk := c.connections[login]

	if !connectionOk && loginOk && r.URL.Path == CallbackPath {
		token, err := c.Authenticator.Token(state, r)

		if err != nil {
			log.Fatal(err)
		}

		tConn := SpotifyClient{c.Authenticator.NewClient(token)}
		connection = &tConn
		c.connections[login] = connection
	}

	page := Page{
		Login:         login,
		Authenticated: (connection != nil),
		AuthUrl:       c.Authenticator.AuthURL(state),
		Flashes:       flashes,
	}

	context.Set(r, "connection", connection)
	context.Set(r, "page", page)

	c.h.ServeHTTP(w, r)
}
