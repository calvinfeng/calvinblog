package youtubeapi

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

func New(cfg Config) Service {
	return &service{
		apiKey: cfg.APIKey,
		host:   "www.googleapis.com",
	}
}

type service struct {
	host   string
	apiKey string
}

func (s *service) PlaylistItems(playlistID string) ([]PlaylistItem, error) {
	u := url.URL{
		Scheme: "https",
		Host:   s.host,
		Path:   "youtube/v3/playlistItems",
	}

	q := u.Query()
	q.Set("part", "snippet,contentDetails")
	q.Set("playlistId", playlistID)
	q.Set("key", s.apiKey)
	u.RawQuery = q.Encode()

	var nextPageToken string
	items := make([]PlaylistItem, 0)
	for {
		if nextPageToken != "" {
			q := u.Query()
			q.Set("pageToken", nextPageToken)
		}
		paginatedItems, nextPageToken, err := getPlaylistItems(u)
		if err != nil {
			return nil, err
		}
		items = append(items, paginatedItems...)
		if nextPageToken == "" {
			break
		}
	}
	return items, nil
}

func getPlaylistItems(resourceURL string) ([]PlaylistItem, string, error) {
	resp, err := http.Get(resourceURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var respPayload PlaylistItemResponse
	if err := json.Unmarshal(body, &respPayload); err != nil {
		return nil, "", err
	}

	return respPayload.Items, respPayload.NextPageToken, nil
}
