package surfforecast

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ztimes2/surfforecast-go/internal/htmlutil"
	"golang.org/x/net/html"
)

func (c *Client) DailyForecast(breakName string) (DailyForecast, error) {
	req, err := newRequest(http.MethodGet, fmt.Sprintf(endpointFormatDailyForecast, breakName))
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not prepare request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return DailyForecast{}, fmt.Errorf("received response with %d status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	node, err := html.Parse(resp.Body)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not parse response as html: %w", err)
	}

	forecast, err := scrapeDailyForecast(node)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape html: %w", err)
	}

	return forecast, nil
}

func scrapeDailyForecast(n *html.Node) (DailyForecast, error) {
	tableNode, ok := htmlutil.Find(n, htmlutil.WithAttributeValue("class", "forecast-table__basic"))
	if !ok {
		return DailyForecast{}, errors.New("could not find table node")
	}

	daysNode, ok := htmlutil.Find(
		tableNode,
		htmlutil.WithAttributeValue("class", "forecast-table__row forecast-table-days"),
		htmlutil.WithAttributeValue("data-row-name", "days"),
	)
	if !ok {
		return DailyForecast{}, errors.New("could not find days node")
	}

	firstDayNode := daysNode.FirstChild
	if firstDayNode == nil {
		return DailyForecast{}, errors.New("could not find first day node")
	}

	firstDay, err := scrapeDay(firstDayNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day: %w", err)
	}

	forecast := DailyForecast{
		Day: firstDay,
	}

	timeNode, ok := htmlutil.Find(
		tableNode,
		htmlutil.WithAttributeValue("class", "forecast-table__row forecast-table-time"),
		htmlutil.WithAttributeValue("data-row-name", "time"),
	)
	if !ok {
		return DailyForecast{}, errors.New("could not find time node")
	}

	if err := htmlutil.ForEach(timeNode, func(n *html.Node) error {
		if htmlutil.AttributeContainsValue(n, "class", "forecast-table__cell") {
			hour, err := scrapeHour(n)
			if err != nil {
				return fmt.Errorf("could not scrape hour: %w", err)
			}

			forecast.HourlyForecasts = append(forecast.HourlyForecasts, HourlyForecast{
				Hour: hour,
			})

			isDayEnd := htmlutil.AttributeContainsValue(n, "class", "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape time data: %w", err)
	}

	ratingNode, ok := htmlutil.Find(
		tableNode,
		htmlutil.WithAttributeValue("class", "forecast-table__row forecast-table-rating"),
		htmlutil.WithAttributeValue("data-row-name", "rating"),
	)
	if !ok {
		return DailyForecast{}, errors.New("could not find rating node")
	}

	var i int
	if err := htmlutil.ForEach(ratingNode, func(n *html.Node) error {
		if htmlutil.AttributeContainsValue(n, "class", "forecast-table__cell") {
			ratingAttr, ok := htmlutil.Attribute(n.FirstChild, "alt")
			if !ok {
				return errors.New("could not find rating attribute")
			}

			rating, err := parseRating(ratingAttr.Val)
			if err != nil {
				return fmt.Errorf("could not parse rating: %w", err)
			}

			forecast.HourlyForecasts[i].Rating = rating

			i++

			isDayEnd := htmlutil.AttributeContainsValue(n, "class", "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape rating data: %w", err)
	}

	waveHeightNode, ok := htmlutil.Find(
		tableNode,
		htmlutil.WithAttributeValue("class", "forecast-table__row"),
		htmlutil.WithAttributeValue("data-row-name", "wave-height"),
	)
	if !ok {
		return DailyForecast{}, errors.New("could not find wave height node")
	}

	i = 0
	if err := htmlutil.ForEach(waveHeightNode, func(n *html.Node) error {
		if htmlutil.AttributeContainsValue(n, "class", "forecast-table__cell") {
			swellAttr, ok := htmlutil.Attribute(n, "data-swell-state")
			if !ok {
				return errors.New("could not find swell attribute")
			}

			var swells []*swell
			if err := json.Unmarshal([]byte(swellAttr.Val), &swells); err != nil {
				return fmt.Errorf("could not unmarshal swell: %w", err)
			}

			for _, s := range swells {
				if s == nil {
					continue
				}

				forecast.HourlyForecasts[i].Swells = append(forecast.HourlyForecasts[i].Swells, Swell{
					PeriodInSeconds:    s.Period,
					Angle:              s.Angle,
					Letters:            s.Letters,
					WaveHeightInMeters: s.Height,
				})
			}

			i++

			isDayEnd := htmlutil.AttributeContainsValue(n, "class", "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape wave data: %w", err)
	}

	return forecast, nil
}

type DailyForecast struct {
	Day             Day
	HourlyForecasts []HourlyForecast
}

type Day struct {
	Weekday  time.Weekday
	MonthDay int
}

type HourlyForecast struct {
	Hour   int
	Rating int
	Swells []Swell
	// TODO wind
	// TODO tide
}

type Swell struct {
	PeriodInSeconds    int
	Angle              float64 // TODO rename to reflect unit of measurement
	Letters            string  // TODO rename to reflect what these letters represent
	WaveHeightInMeters float64
}

func scrapeDay(n *html.Node) (Day, error) {
	container := n.LastChild
	if container == nil {
		return Day{}, errors.New("container node not found")
	}

	weekdayContainer := container.FirstChild
	if weekdayContainer == nil {
		return Day{}, errors.New("weekday container node not found")
	}

	weekdayText := weekdayContainer.FirstChild
	if weekdayText == nil {
		return Day{}, errors.New("weekday text node not found")
	}

	weekday, err := parseWeekday(weekdayText.Data)
	if err != nil {
		return Day{}, fmt.Errorf("could not parse weekday: %w", err)
	}

	monthDayContainer := container.LastChild
	if monthDayContainer == nil {
		return Day{}, errors.New("month day container node not found")
	}

	monthDayText := monthDayContainer.FirstChild
	if monthDayText == nil {
		return Day{}, errors.New("month day text node not found")
	}

	monthDay, err := parseMonthDay(monthDayText.Data)
	if err != nil {
		return Day{}, fmt.Errorf("could not parse month day: %w", err)
	}

	return Day{
		Weekday:  weekday,
		MonthDay: monthDay,
	}, nil
}

func parseWeekday(s string) (time.Weekday, error) {
	switch s {
	case "Sunday":
		return time.Sunday, nil
	case "Monday":
		return time.Monday, nil
	case "Tuesday":
		return time.Tuesday, nil
	case "Wednesday":
		return time.Wednesday, nil
	case "Thursday":
		return time.Thursday, nil
	case "Friday":
		return time.Friday, nil
	case "Saturday":
		return time.Saturday, nil
	default:
		return time.Weekday(0), fmt.Errorf("invalid weekday: %q", s)
	}
}

func parseMonthDay(s string) (int, error) {
	day, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not integer: %q", s)
	}

	if day < 0 || day > 31 {
		return 0, fmt.Errorf("not month day: %q", s)
	}

	return day, nil
}

func scrapeHour(n *html.Node) (int, error) {
	hourContainer := n.FirstChild
	if hourContainer == nil {
		return 0, errors.New("hour container node not found")
	}

	hourText := hourContainer.FirstChild
	if hourText == nil {
		return 0, errors.New("hour text node not found")
	}

	hour, err := parseTwelveClockHour(hourText.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse hour: %w", err)
	}

	periodContainer := n.LastChild
	if periodContainer == nil {
		return 0, errors.New("clock period node not found")
	}

	periodText := periodContainer.FirstChild
	if periodText == nil {
		return 0, errors.New("clock period text not found")
	}

	period, err := parseClockPeriod(periodText.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse clock period: %w", err)
	}

	return toTwentyFourClockHour(hour, period), nil
}

func parseTwelveClockHour(s string) (int, error) {
	hour, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not integer: %q", s)
	}

	if hour < 1 || hour > 12 {
		return 0, fmt.Errorf("not 12 clock hour: %q", s)
	}

	return hour, nil
}

type clockPeriod int

const (
	beforeMidday clockPeriod = iota
	afterMidday
)

func parseClockPeriod(s string) (clockPeriod, error) {
	switch s {
	case "AM":
		return beforeMidday, nil
	case "PM":
		return afterMidday, nil
	default:
		return clockPeriod(0), fmt.Errorf("invalid clock period: %q", s)
	}
}

func toTwentyFourClockHour(hour int, p clockPeriod) int {
	if p == beforeMidday {
		if hour == 12 {
			return 0
		}
		return hour
	}
	if hour == 12 {
		return hour
	}
	return hour + 12
}

func parseRating(s string) (int, error) {
	rating, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not integer: %q", s)
	}

	if rating < 0 || rating > 10 {
		return 0, fmt.Errorf("invalid rating: %q", s)
	}

	return rating, nil
}

type swell struct {
	Period  int     `json:"period"`
	Angle   float64 `json:"angle"`
	Letters string  `json:"letters"`
	Height  float64 `json:"height"`
}
