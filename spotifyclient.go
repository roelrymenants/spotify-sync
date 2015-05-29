package main

import (
	"time"

	"github.com/zmb3/spotify"
)

type SpotifyClient struct {
	spotify.Client
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
		time.Sleep(1 * time.Second)

		_, err = c.getLibraryTrackListRec(offset+limit, trackList)
		if err != nil {
			return nil, err
		}
	}

	return trackList, nil
}
