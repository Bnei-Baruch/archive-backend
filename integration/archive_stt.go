package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

// ArchiveSttService is archive-stt (markup of content units' media): the heard words of content units with no
// transcript, for their content in the index.
type ArchiveSttService interface {
	// ContentUnits are the content units whose markup has no transcript: uid -> its language -> true.
	ContentUnits() (map[string]map[string]bool, error)
	// Content is the content unit's heard words as plain text, paragraphs apart by an empty line.
	Content(uid, language string) (string, error)
}

type DefaultArchiveSttService struct {
	client  *http.Client
	baseUrl string
}

func NewArchiveSttService(url string) ArchiveSttService {
	return &DefaultArchiveSttService{
		baseUrl: url,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (s *DefaultArchiveSttService) get(path string) ([]byte, error) {
	resp, err := s.client.Get(s.baseUrl + path)
	if err != nil {
		return nil, errors.Wrap(err, "client.Get")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("archive-stt %s: %s", path, resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	return body, nil
}

func (s *DefaultArchiveSttService) ContentUnits() (map[string]map[string]bool, error) {
	body, err := s.get("/cus")
	if err != nil {
		return nil, err
	}
	var units []struct {
		UID      string `json:"uid"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(body, &units); err != nil {
		return nil, errors.Wrap(err, "json.Unmarshal")
	}
	out := make(map[string]map[string]bool, len(units))
	for _, l := range units {
		if out[l.UID] == nil {
			out[l.UID] = make(map[string]bool)
		}
		out[l.UID][l.Language] = true
	}
	return out, nil
}

func (s *DefaultArchiveSttService) Content(uid, language string) (string, error) {
	body, err := s.get(fmt.Sprintf("/cus/%s/content?lang=%s", url.PathEscape(uid), url.QueryEscape(language)))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
