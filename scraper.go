package surfforecast

import (
	"net/http"
	"time"

	"github.com/tkuchiki/go-timezone"
)

const (
	baseURL = "https://www.surf-forecast.com"
)

const (
	defaultRequestTimeout = 10 * time.Second
)

// Scraper is a web scraper that sends requests to www.surf-forecast.com and scrapes
// data from its responses.
type Scraper struct {
	httpClient *http.Client
	timezones  *timezone.Timezone
	baseURL    string
}

// New initializes a new Scraper.
func New(opts ...Option) *Scraper {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	return &Scraper{
		httpClient: o.resolveHTTPClient(),
		timezones:  o.resolveTimezones(),
		baseURL:    baseURL,
	}
}

// Option is an optional function for configuring a Scraper.
type Option func(*options)

// options holds all the options available for configuring a Scraper.
type options struct {
	httpClient *http.Client
	timezones  *timezone.Timezone
	// TODO allow authentication to fetch even more detailed reports
}

// resolveHTTPClient returns either a custom HTTP client or the default one in case
// if no custom client was provided.
func (o options) resolveHTTPClient() *http.Client {
	if o.httpClient != nil {
		return o.httpClient
	}
	return &http.Client{
		Timeout: defaultRequestTimeout,
	}
}

func (o options) resolveTimezones() *timezone.Timezone {
	if o.timezones != nil {
		return o.timezones
	}
	return timezone.New()
}

// WithHTTPClient sets a custom HTTP client for Scraper.
func WithHTTPClient(c *http.Client) Option {
	return func(o *options) {
		o.httpClient = c
	}
}

// WithTimezone sets a custom timezone.Timezone for Scraper.
func WithTimezone(t *timezone.Timezone) Option {
	return func(o *options) {
		o.timezones = t
	}
}
