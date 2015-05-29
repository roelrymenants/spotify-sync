package main

import (
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

func newConnectionHandler(host string, clientId string, clientSecret string) *connectionHandler {
	redirectUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   CallbackPath,
	}

	auth := spotify.NewAuthenticator(redirectUrl.String(), spotify.ScopeUserLibraryRead, spotify.ScopeUserLibraryModify)
	auth.SetAuthInfo(clientId, clientSecret)

	return &connectionHandler{
		h:             http.DefaultServeMux,
		clientId:      clientId,
		clientSecret:  clientSecret,
		Authenticator: &auth,
	}
}

func (c connectionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "spotify-sync")

	login, loginOk := session.Values["login"].(string)

	imported, _ := session.Values["imported"].(bool)

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
		Imported:      imported,
		Authenticated: (connection != nil),
	}

	context.Set(r, "connection", connection)
	context.Set(r, "page", page)

	c.h.ServeHTTP(w, r)
}
