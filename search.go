package surfforecast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	pathSearchBreaks = "/breaks/ac_location_name"
)

func (s *Scraper) SearchBreaks(query string) ([]BreakSearchResult, error) {
	u, err := url.Parse(baseURL + pathSearchBreaks)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request url: %w", err)
	}

	vals := url.Values{}
	vals.Add("query", query)
	u.RawQuery = vals.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received response with %d status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %w", err)
	}

	body = bytes.ReplaceAll(body, []byte(`'`), []byte(`"`))

	var rawResults [][]string
	if err := json.Unmarshal(body, &rawResults); err != nil {
		return nil, fmt.Errorf("could not unmarshal response body: %w", err)
	}

	var results []BreakSearchResult
	for _, r := range rawResults {
		if len(r) != 3 {
			return nil, fmt.Errorf("unexpected search result")
		}

		results = append(results, BreakSearchResult{
			Name:        r[1],
			CountryName: r[2],
		})
	}

	return results, nil
}

type BreakSearchResult struct {
	Name        string
	CountryName string
}
