package surfforecast

import "net/http"

const (
	baseURL = "https://www.surf-forecast.com"

	endpointFormatDailyForecast  = "/breaks/%s/forecasts/latest"
	endpointFormatWeeklyForecast = "/breaks/%s/forecasts/latest/six_days"
)

type Client struct {
	httpClient *http.Client
}

func New(opts ...Option) *Client {
	o := Options{
		httpClient: &http.Client{
			// TODO configure
		},
	}

	for _, opt := range opts {
		opt(&o)
	}

	return &Client{
		httpClient: o.httpClient,
	}
}

type Option func(*Options)

type Options struct {
	httpClient *http.Client
}

func newRequest(method, path string) (*http.Request, error) {
	return http.NewRequest(method, baseURL+path, nil)
}
