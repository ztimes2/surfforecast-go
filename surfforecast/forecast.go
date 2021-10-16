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

var ErrBreakNotFound = errors.New("break not found")

func (c *Client) DailyForecast(breakName string) (DailyForecast, error) {
	// TODO use chromedp to dynamically expand first day's forecast

	req, err := newRequest(http.MethodGet, fmt.Sprintf(endpointFormatDailyForecast, breakName))
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not prepare request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return DailyForecast{}, ErrBreakNotFound
		}
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

type DailyForecast struct {
	Day             Day // TODO convert to time.Time
	HourlyForecasts []HourlyForecast
}

func newDailyForecast(
	day Day,
	hours []int,
	ratings []int,
	swells [][]Swell,
	waveEnergies []float64) (DailyForecast, error) {

	if len(hours) != len(ratings) {
		return DailyForecast{}, errors.New("hours and ratings must have equal number of elements")
	}

	if len(hours) != len(swells) {
		return DailyForecast{}, errors.New("hours and swells must have equal number of elements")
	}

	if len(hours) != len(waveEnergies) {
		return DailyForecast{}, errors.New("hours and wave energies must have equal number of elements")
	}

	hourlyForecasts := make([]HourlyForecast, len(hours))
	for i := range hourlyForecasts {
		hourlyForecasts[i].Hour = hours[i]
		hourlyForecasts[i].Rating = ratings[i]
		hourlyForecasts[i].Swells = swells[i]
		hourlyForecasts[i].WaveEnergyInKiloJoules = waveEnergies[i]
	}

	return DailyForecast{
		Day:             day,
		HourlyForecasts: hourlyForecasts,
	}, nil
}

type Day struct {
	Weekday  time.Weekday
	MonthDay int
}

type HourlyForecast struct {
	Hour                   int // TODO convert to time.Time
	Rating                 int
	Swells                 []Swell
	WaveEnergyInKiloJoules float64
	// TODO wind
	// TODO tide
}

type Swell struct {
	PeriodInSeconds          float64
	DirectionInDegrees       float64
	DirectionInCompassPoints string
	WaveHeightInMeters       float64
}

func scrapeDailyForecast(n *html.Node) (DailyForecast, error) {
	tableNode, ok := htmlutil.Find(n, htmlutil.WithClassEquals("forecast-table__basic"))
	if !ok {
		return DailyForecast{}, errors.New("could not find table node")
	}

	firstDay, err := scrapeFirstDay(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day: %w", err)
	}

	firstDayHours, err := scrapeFirstDayHours(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day hours: %w", err)
	}

	firstDayRatings, err := scrapeFirstDayRatings(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day ratings: %w", err)
	}

	firstDaySwells, err := scrapeFirstDaySwells(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day swells: %w", err)
	}

	firstDayWaveEnergies, err := scrapeFirstDayWaveEnergies(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day wave energies: %w", err)
	}

	return newDailyForecast(
		firstDay,
		firstDayHours,
		firstDayRatings,
		firstDaySwells,
		firstDayWaveEnergies,
	)
}

func scrapeFirstDay(n *html.Node) (Day, error) {
	daysNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEquals("forecast-table__row forecast-table-days"),
		htmlutil.WithAttributeEquals("data-row-name", "days"),
	)
	if !ok {
		return Day{}, errors.New("could not find days node")
	}

	firstDayNode := daysNode.FirstChild
	if firstDayNode == nil {
		return Day{}, errors.New("could not find first day node")
	}

	firstDay, err := scrapeDay(firstDayNode)
	if err != nil {
		return Day{}, fmt.Errorf("could not scrape day: %w", err)
	}

	return firstDay, nil
}

func scrapeDay(n *html.Node) (Day, error) {
	container := n.LastChild
	if container == nil {
		return Day{}, errors.New("could not find day container node")
	}

	weekdayContainer := container.FirstChild
	if weekdayContainer == nil {
		return Day{}, errors.New("could not find weekday container node")
	}

	weekdayText := weekdayContainer.FirstChild
	if weekdayText == nil {
		return Day{}, errors.New("could not find weekday text node")
	}

	weekday, err := parseWeekday(weekdayText.Data)
	if err != nil {
		return Day{}, fmt.Errorf("could not parse weekday: %w", err)
	}

	monthDayContainer := container.LastChild
	if monthDayContainer == nil {
		return Day{}, errors.New("could not find month day container node")
	}

	monthDayText := monthDayContainer.FirstChild
	if monthDayText == nil {
		return Day{}, errors.New("could not find month day text node")
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

func scrapeFirstDayHours(n *html.Node) ([]int, error) {
	hoursNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEquals("forecast-table__row forecast-table-time"),
		htmlutil.WithAttributeEquals("data-row-name", "time"),
	)
	if !ok {
		return nil, errors.New("could not find hours node")
	}

	var hours []int
	if err := htmlutil.ForEach(hoursNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, "forecast-table__cell") {
			hour, err := scrapeHour(n)
			if err != nil {
				return fmt.Errorf("could not scrape hour: %w", err)
			}

			hours = append(hours, hour)

			isDayEnd := htmlutil.ClassContains(n, "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape hours: %w", err)
	}

	return hours, nil
}

func scrapeHour(n *html.Node) (int, error) {
	hourContainer := n.FirstChild
	if hourContainer == nil {
		return 0, errors.New("could not find hour container node")
	}

	hourText := hourContainer.FirstChild
	if hourText == nil {
		return 0, errors.New("could not find hour text node")
	}

	hour, err := parseTwelveClockHour(hourText.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse hour: %w", err)
	}

	periodContainer := n.LastChild
	if periodContainer == nil {
		return 0, errors.New("could not find clock period node")
	}

	periodText := periodContainer.FirstChild
	if periodText == nil {
		return 0, errors.New("could not find clock period text node")
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

func scrapeFirstDayRatings(n *html.Node) ([]int, error) {
	ratingsNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEquals("forecast-table__row forecast-table-rating"),
		htmlutil.WithAttributeEquals("data-row-name", "rating"),
	)
	if !ok {
		return nil, errors.New("could not find ratings node")
	}

	var ratings []int
	if err := htmlutil.ForEach(ratingsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, "forecast-table__cell") {
			ratingAttr, ok := htmlutil.Attribute(n.FirstChild, "alt")
			if !ok {
				return errors.New("could not find rating attribute")
			}

			rating, err := parseRating(ratingAttr.Val)
			if err != nil {
				return fmt.Errorf("could not parse rating: %w", err)
			}

			ratings = append(ratings, rating)

			isDayEnd := htmlutil.ClassContains(n, "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape ratings: %w", err)
	}

	return ratings, nil
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

func scrapeFirstDayWaveEnergies(n *html.Node) ([]float64, error) {
	energiesNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEquals("forecast-table__row"),
		htmlutil.WithAttributeEquals("data-row-name", "energy"),
	)
	if !ok {
		return nil, errors.New("could not find wave energies node")
	}

	var energies []float64
	if err := htmlutil.ForEach(energiesNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, "forecast-table__cell") {
			energy, err := scrapeWaveEnergy(n)
			if err != nil {
				return fmt.Errorf("could not scrape wave energy: %w", err)
			}

			energies = append(energies, energy)

			isDayEnd := htmlutil.ClassContains(n, "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape wave energies: %w", err)
	}

	return energies, nil
}

func scrapeWaveEnergy(n *html.Node) (float64, error) {
	container := n.FirstChild
	if container == nil {
		return 0, errors.New("could not find wave energy container node")
	}

	energyText := container.FirstChild
	if energyText == nil {
		return 0, errors.New("could not find wave energy text node")
	}

	energy, err := parseWaveEnergy(energyText.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse wave energy: %w", err)
	}

	return energy, nil
}

func parseWaveEnergy(s string) (float64, error) {
	energy, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not float: %q", s)
	}

	if energy < 0 {
		return 0, fmt.Errorf("invalid wave energy: %q", s)
	}

	return energy, nil
}

func scrapeFirstDaySwells(n *html.Node) ([][]Swell, error) {
	swellsNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEquals("forecast-table__row"),
		htmlutil.WithAttributeEquals("data-row-name", "wave-height"),
	)
	if !ok {
		return nil, errors.New("could not find swells node")
	}

	var swells [][]Swell
	if err := htmlutil.ForEach(swellsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, "forecast-table__cell") {
			swellAttr, ok := htmlutil.Attribute(n, "data-swell-state")
			if !ok {
				return errors.New("could not find swell attribute")
			}

			ss, err := unmarshalSwells([]byte(swellAttr.Val))
			if err != nil {
				return fmt.Errorf("could not unmarshal swells: %w", err)
			}

			swells = append(swells, ss)

			isDayEnd := htmlutil.ClassContains(n, "is-day-end")
			if isDayEnd {
				return htmlutil.ErrLoopStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape swells: %w", err)
	}

	return swells, nil
}

func unmarshalSwells(b []byte) ([]Swell, error) {
	var payloads []*swellPayload
	if err := json.Unmarshal(b, &payloads); err != nil {
		return nil, fmt.Errorf("could not unmarshal swell: %w", err)
	}

	var swells []Swell
	for _, p := range payloads {
		if p == nil {
			continue
		}

		swells = append(swells, Swell{
			PeriodInSeconds:          p.Period,
			DirectionInDegrees:       p.Angle,
			DirectionInCompassPoints: p.Letters,
			WaveHeightInMeters:       p.Height,
		})
	}

	return swells, nil
}

type swellPayload struct {
	Period  float64 `json:"period"`
	Angle   float64 `json:"angle"`
	Letters string  `json:"letters"`
	Height  float64 `json:"height"`
}
