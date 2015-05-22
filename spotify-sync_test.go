package main

import (
	"encoding/json"
	"testing"
)

func TestUnmarshall(t *testing.T) {
	jsonBlob := "{\"items\": [{\"track\": {\"name\": \"test\"}}]}"

	data := struct {
		Items []struct {
			Track struct {
				Name string
			}
		}
	}{}

	err := json.Unmarshal([]byte(jsonBlob), &data)

	if err != nil {
		t.Error(err)
	}

	if data.Items[0].Track.Name != "test" {
		t.Errorf("Got name: '%v', expected 'test'", data.Items[0].Track.Name)
	}
}
