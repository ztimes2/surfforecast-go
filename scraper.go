package surfforecast

import (
	"net/http"
	"time"

	"github.com/tkuchiki/go-timezone"
)

const (
	baseURL = "https://www.surf-forecast.com"

	pathFormatForecastsForEightDays  = "/breaks/%s/forecasts/latest"
	pathFormatForecastsForTwelveDays = "/breaks/%s/forecasts/latest/six_days"
)

const (
	defaultRequestTimeout = 10 * time.Second
)

type Scraper struct {
	httpClient *http.Client
	timezones  *timezone.Timezone
}

func New(opts ...Option) *Scraper {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}

	return &Scraper{
		httpClient: o.resolveHTTPClient(),
		timezones:  timezone.New(),
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
