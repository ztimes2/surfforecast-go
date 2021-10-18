package surfforecast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ztimes2/surfforecast-go/internal/htmlutil"
	"golang.org/x/net/html"
)

const (
	pathSearchBreaks = "/breaks/ac_location_name"
	pathFormatBreak  = "/breaks/%s"

	queryParamSearchQuery = "query"
)

const (
	idDropFormControlNav   = "dropformcont-nav"
	idCountry              = "country_id"
	idLocationFilenamePart = "location_filename_part"

	attributeSelected = "selected"
)

var (
	ErrBreakNotFound = errors.New("break not found")
)

func (s *Scraper) SearchBreaks(query string) ([]Break, error) {
	u, err := url.Parse(baseURL + pathSearchBreaks)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request url: %w", err)
	}

	vals := url.Values{}
	vals.Add(queryParamSearchQuery, query)
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

	var results [][]string
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("could not unmarshal response body: %w", err)
	}

	var breaks []Break
	for _, result := range results {
		if len(result) != 3 {
			return nil, fmt.Errorf("unexpected search result")
		}

		breaks = append(breaks, Break{
			Name:        result[1],
			CountryName: result[2],
		})
	}

	return breaks, nil
}

type Break struct {
	Name        string
	CountryName string
}

func (s *Scraper) Break(breakName string) (Break, error) {
	path := fmt.Sprintf(pathFormatBreak, breakName)

	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return Break{}, fmt.Errorf("could not prepare request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Break{}, fmt.Errorf("could not send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return Break{}, ErrBreakNotFound
		}
		return Break{}, fmt.Errorf("received response with %d status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	node, err := html.Parse(resp.Body)
	if err != nil {
		return Break{}, fmt.Errorf("could not parse response body as html: %w", err)
	}

	brk, err := scrapeBreak(node)
	if err != nil {
		return Break{}, fmt.Errorf("could not scrape break: %w", err)
	}

	return brk, nil
}

func scrapeBreak(n *html.Node) (Break, error) {
	navNode, ok := htmlutil.FindOne(n, htmlutil.WithIDEqual(idDropFormControlNav))
	if !ok {
		return Break{}, errors.New("could not find navigation node")
	}

	countryNode, ok := htmlutil.FindOne(navNode, htmlutil.WithIDEqual(idCountry))
	if !ok {
		return Break{}, errors.New("could not find country node")
	}

	countryNameNode, ok := htmlutil.FindOne(countryNode, htmlutil.WithAttribute(attributeSelected))
	if !ok {
		return Break{}, errors.New("could not find country name node")
	}

	countryNameTextNode := countryNameNode.FirstChild
	if countryNameTextNode == nil {
		return Break{}, errors.New("could not find country name text node")
	}

	breakNode, ok := htmlutil.FindOne(navNode, htmlutil.WithIDEqual(idLocationFilenamePart))
	if !ok {
		return Break{}, errors.New("could not find break node")
	}

	breakNameNode, ok := htmlutil.FindOne(breakNode, htmlutil.WithAttribute(attributeSelected))
	if !ok {
		return Break{}, errors.New("could not find break name node")
	}

	breakNameTextNode := breakNameNode.FirstChild
	if countryNameTextNode == nil {
		return Break{}, errors.New("could not find break name text node")
	}

	return Break{
		Name:        breakNameTextNode.Data,
		CountryName: countryNameTextNode.Data,
	}, nil
}
