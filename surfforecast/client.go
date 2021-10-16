package surfforecast

import (
	"net/http"
	"time"
)

const (
	baseURL = "https://www.surf-forecast.com"

	endpointFormatDailyForecast  = "/breaks/%s/forecasts/latest"
	endpointFormatWeeklyForecast = "/breaks/%s/forecasts/latest/six_days"
)

const (
	defaultRequestTimeout = 10 * time.Second
)

type Client struct {
	httpClient *http.Client
}

func New(opts ...Option) *Client {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}

	return &Client{
		httpClient: o.resolveHTTPClient(),
	}
}

type Option func(*Options)

type Options struct {
	httpClient *http.Client
	// TODO allow authentication to fetch even more detailed reports
}

func (o Options) resolveHTTPClient() *http.Client {
	if o.httpClient != nil {
		return o.httpClient
	}
	return &http.Client{
		Timeout: defaultRequestTimeout,
	}
}

func newRequest(method, path string) (*http.Request, error) {
	return http.NewRequest(method, baseURL+path, nil)
}
