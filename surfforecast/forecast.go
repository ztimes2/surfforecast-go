package surfforecast

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tkuchiki/go-timezone"
	"github.com/ztimes2/surfforecast-go/internal/htmlutil"
	"golang.org/x/net/html"
)

const (
	classBreakHeaderIssued   = "break-header__issued"
	classForecastTableBasic  = "forecast-table__basic"
	classForecastTableRow    = "forecast-table__row"
	classForecastTableCell   = "forecast-table__cell"
	classForecastTableTime   = "forecast-table-time"
	classForecastTableDays   = "forecast-table-days"
	classForecastTableRating = "forecast-table-rating"
	classIsDayEnd            = "is-day-end"

	attributeDataRowName        = "data-row-name"
	attributeDataSwellState     = "data-swell-state"
	attributeDataSpeed          = "data-speed"
	attributeAlternateImageText = "alt"
	attributeTransform          = "transform"

	dataRowNameDays       = "days"
	dataRowNameTime       = "time"
	dataRowNameRating     = "rating"
	dataRowNameWaveHeight = "wave-height"
	dataRowNameEnergy     = "energy"
	dataRowNameWind       = "wind"
	dataRowNameWindState  = "wind-state"

	transformRotatePrefix = "rotate("
	transformRotateSuffix = ")"
)

var ErrBreakNotFound = errors.New("break not found")

func (c *Client) DailyForecast(breakName string) (DailyForecast, error) {
	// TODO enable context propogation and cancelation
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

	forecast, err := scrapeDailyForecast(node, c.timezones)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape html: %w", err)
	}

	return forecast, nil
}

type DailyForecast struct {
	Date            time.Time
	HourlyForecasts []HourlyForecast
}

func newDailyForecast(
	issueDate time.Time,
	day int,
	hours []int,
	ratings []int,
	swells [][]Swell,
	waveEnergies []float64,
	winds []Wind) (DailyForecast, error) {

	if len(hours) != len(ratings) {
		return DailyForecast{}, errors.New("hours and ratings must have equal number of elements")
	}
	if len(hours) != len(swells) {
		return DailyForecast{}, errors.New("hours and swells must have equal number of elements")
	}
	if len(hours) != len(waveEnergies) {
		return DailyForecast{}, errors.New("hours and wave energies must have equal number of elements")
	}
	if len(hours) != len(winds) {
		return DailyForecast{}, errors.New("hours and winds must have equal number of elements")
	}

	date := time.Date(issueDate.Year(), issueDate.Month(), day, 0, 0, 0, 0, issueDate.Location())

	hourlyForecasts := make([]HourlyForecast, len(hours))
	for i := range hourlyForecasts {
		hourlyForecasts[i].Date = time.Date(date.Year(), date.Month(), date.Day(), hours[i], 0, 0, 0, date.Location())
		hourlyForecasts[i].Rating = ratings[i]
		hourlyForecasts[i].Swells = swells[i]
		hourlyForecasts[i].WaveEnergyInKiloJoules = waveEnergies[i]
		hourlyForecasts[i].Wind = winds[i]
	}

	return DailyForecast{
		Date:            date,
		HourlyForecasts: hourlyForecasts,
	}, nil
}

type HourlyForecast struct {
	Date                   time.Time
	Rating                 int
	Swells                 []Swell
	WaveEnergyInKiloJoules float64
	Wind                   Wind
	// TODO tide
}

type Swell struct {
	PeriodInSeconds          float64
	DirectionInDegrees       float64
	DirectionInCompassPoints string
	WaveHeightInMeters       float64
}

type Wind struct {
	SpeedInKilometersPerHour float64
	DirectionInDegrees       float64
	DirectionInCompassPoints string
	State                    string
}

func scrapeDailyForecast(n *html.Node, tz *timezone.Timezone) (DailyForecast, error) {
	issueDate, err := scrapeIssueDate(n, tz)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape issue date: %w", err)
	}

	tableNode, ok := htmlutil.Find(n, htmlutil.WithClassEqual(classForecastTableBasic))
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

	firstDayWinds, err := scrapeFirstDayWinds(tableNode)
	if err != nil {
		return DailyForecast{}, fmt.Errorf("could not scrape first day winds: %w", err)
	}

	return newDailyForecast(
		issueDate,
		firstDay,
		firstDayHours,
		firstDayRatings,
		firstDaySwells,
		firstDayWaveEnergies,
		firstDayWinds,
	)
}

func scrapeIssueDate(n *html.Node, tz *timezone.Timezone) (time.Time, error) {
	container, ok := htmlutil.Find(n, htmlutil.WithClassEqual(classBreakHeaderIssued))
	if !ok {
		return time.Time{}, errors.New("could not find issue container node")
	}

	text := container.FirstChild
	if text == nil {
		return time.Time{}, errors.New("could not find issue text node")
	}

	parts := strings.Split(text.Data, " ")
	if len(parts) != 12 {
		return time.Time{}, fmt.Errorf("unexpected issue text: %q", text.Data)
	}

	dayText, monthText, yearText, tzAbbr := parts[8], parts[9], parts[10], parts[11]

	day, err := parseDay(dayText)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse issue day: %w", err)
	}

	month, err := parseMonthShort(monthText)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse issue month: %w", err)
	}

	year, err := strconv.Atoi(yearText)
	if err != nil {
		return time.Time{}, fmt.Errorf("issue year not integer: %q", yearText)
	}

	timezones, err := tz.GetTimezones(tzAbbr)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not find timezones for %q abbreviation: %w", tzAbbr, err)
	}

	if len(timezones) == 0 {
		return time.Time{}, fmt.Errorf("0 timezones found for %q abbreviation", tzAbbr)
	}

	timezone := timezones[0]

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not find time location for %q", timezone)
	}

	return time.Date(year, month, day, 0, 0, 0, 0, loc), nil
}

func parseDay(s string) (int, error) {
	day, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not integer: %q", s)
	}

	if day < 0 || day > 31 {
		return 0, fmt.Errorf("not month day: %q", s)
	}

	return day, nil
}

func parseMonthShort(s string) (time.Month, error) {
	switch s {
	case "Jan":
		return time.January, nil
	case "Feb":
		return time.February, nil
	case "Mar":
		return time.March, nil
	case "Apr":
		return time.April, nil
	case "May":
		return time.May, nil
	case "Jun":
		return time.June, nil
	case "Jul":
		return time.July, nil
	case "Aug":
		return time.August, nil
	case "Sep":
		return time.September, nil
	case "Oct":
		return time.October, nil
	case "Nov":
		return time.November, nil
	case "Dec":
		return time.December, nil
	default:
		return time.Month(0), fmt.Errorf("invalid short month: %q", s)
	}
}

func scrapeFirstDay(n *html.Node) (int, error) {
	daysNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableDays),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameDays),
	)
	if !ok {
		return 0, errors.New("could not find days node")
	}

	firstDayNode := daysNode.FirstChild
	if firstDayNode == nil {
		return 0, errors.New("could not find first day node")
	}

	firstDay, err := scrapeDay(firstDayNode)
	if err != nil {
		return 0, fmt.Errorf("could not scrape day: %w", err)
	}

	return firstDay, nil
}

func scrapeDay(n *html.Node) (int, error) {
	container := n.LastChild
	if container == nil {
		return 0, errors.New("could not find day container node")
	}

	monthDayContainer := container.LastChild
	if monthDayContainer == nil {
		return 0, errors.New("could not find month day container node")
	}

	monthDayText := monthDayContainer.FirstChild
	if monthDayText == nil {
		return 0, errors.New("could not find month day text node")
	}

	monthDay, err := parseDay(monthDayText.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse month day: %w", err)
	}

	return monthDay, nil
}

func scrapeFirstDayHours(n *html.Node) ([]int, error) {
	hoursNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableTime),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameTime),
	)
	if !ok {
		return nil, errors.New("could not find hours node")
	}

	var hours []int
	if err := htmlutil.ForEach(hoursNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			hour, err := scrapeHour(n)
			if err != nil {
				return fmt.Errorf("could not scrape hour: %w", err)
			}

			hours = append(hours, hour)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
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
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableRating),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameRating),
	)
	if !ok {
		return nil, errors.New("could not find ratings node")
	}

	var ratings []int
	if err := htmlutil.ForEach(ratingsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			ratingAttr, ok := htmlutil.Attribute(n.FirstChild, attributeAlternateImageText)
			if !ok {
				return errors.New("could not find rating attribute")
			}

			rating, err := parseRating(ratingAttr.Val)
			if err != nil {
				return fmt.Errorf("could not parse rating: %w", err)
			}

			ratings = append(ratings, rating)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
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

func scrapeFirstDaySwells(n *html.Node) ([][]Swell, error) {
	swellsNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWaveHeight),
	)
	if !ok {
		return nil, errors.New("could not find swells node")
	}

	var swells [][]Swell
	if err := htmlutil.ForEach(swellsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			hourlySwells, err := scrapeHourlySwells(n)
			if err != nil {
				return fmt.Errorf("could not scrape hourly swells: %w", err)
			}

			swells = append(swells, hourlySwells)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape swells: %w", err)
	}

	return swells, nil
}

func scrapeHourlySwells(n *html.Node) ([]Swell, error) {
	attr, ok := htmlutil.Attribute(n, attributeDataSwellState)
	if !ok {
		return nil, errors.New("could not find swells attribute")
	}

	swells, err := unmarshalSwells([]byte(attr.Val))
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal swells: %w", err)
	}

	return swells, nil
}

func unmarshalSwells(b []byte) ([]Swell, error) {
	var payload []*swell
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %w", err)
	}

	var swells []Swell
	for _, p := range payload {
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

type swell struct {
	Period  float64 `json:"period"`
	Angle   float64 `json:"angle"`
	Letters string  `json:"letters"`
	Height  float64 `json:"height"`
}

func scrapeFirstDayWaveEnergies(n *html.Node) ([]float64, error) {
	energiesNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameEnergy),
	)
	if !ok {
		return nil, errors.New("could not find wave energies node")
	}

	var energies []float64
	if err := htmlutil.ForEach(energiesNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			energy, err := scrapeWaveEnergy(n)
			if err != nil {
				return fmt.Errorf("could not scrape wave energy: %w", err)
			}

			energies = append(energies, energy)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
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

func scrapeFirstDayWinds(n *html.Node) ([]Wind, error) {
	windsNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWind),
	)
	if !ok {
		return nil, errors.New("could not find winds node")
	}

	var winds []Wind
	if err := htmlutil.ForEach(windsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			wind, err := scrapeWind(n)
			if err != nil {
				return fmt.Errorf("could not scrape wind: %w", err)
			}

			winds = append(winds, wind)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape winds: %w", err)
	}

	states, err := scrapeFirstDayWindStates(n)
	if err != nil {
		return nil, fmt.Errorf("could not scrapre first day wind states: %w", err)
	}

	if len(winds) != len(states) {
		return nil, fmt.Errorf("winds and states must have equal number of elements")
	}

	for i := range winds {
		winds[i].State = states[i]
	}

	return winds, nil
}

func scrapeWind(n *html.Node) (Wind, error) {
	container := n.FirstChild
	if container == nil {
		return Wind{}, errors.New("could not find wind container node")
	}

	speedAttr, ok := htmlutil.Attribute(container, attributeDataSpeed)
	if !ok {
		return Wind{}, errors.New("could not find wind speed attribute")
	}

	speed, err := parseWindSpeed(speedAttr.Val)
	if err != nil {
		return Wind{}, fmt.Errorf("could not parse wind speed: %w", err)
	}

	degrees, err := scrapeWindDirectionDegrees(container)
	if err != nil {
		return Wind{}, fmt.Errorf("could not scrape wind direction degrees: %w", err)
	}

	compassContainer := container.LastChild
	if compassContainer == nil {
		return Wind{}, errors.New("could not find wind direction compass container node")
	}

	compassText := compassContainer.FirstChild
	if compassText == nil {
		return Wind{}, errors.New("could not find wind direction compass text node")
	}

	return Wind{
		SpeedInKilometersPerHour: speed,
		DirectionInDegrees:       degrees,
		DirectionInCompassPoints: compassText.Data,
	}, nil
}

func scrapeWindDirectionDegrees(n *html.Node) (float64, error) {
	container := n.FirstChild
	if container == nil {
		return 0, errors.New("could not find wind direction degrees container")
	}

	circle := container.FirstChild
	if circle == nil {
		return 0, errors.New("could not find wind direction circle node")
	}

	arrow := circle.NextSibling
	if arrow == nil {
		return 0, errors.New("could not find wind direction arrow node")
	}

	attr, ok := htmlutil.Attribute(arrow, attributeTransform)
	if !ok {
		return 0, errors.New("could not find transform attribute")
	}

	degreesText := strings.TrimPrefix(attr.Val, transformRotatePrefix)
	degreesText = strings.TrimSuffix(degreesText, transformRotateSuffix)

	degrees, err := parseWindDirectionDegrees(degreesText)
	if err != nil {
		return 0, fmt.Errorf("could not parse wind direction degrees: %w", err)
	}

	return degrees, nil
}

func parseWindDirectionDegrees(s string) (float64, error) {
	degrees, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not float: %q", s)
	}

	if degrees < 0 || degrees > 360 {
		return 0, fmt.Errorf("invalid wind direction degrees: %q", s)
	}

	return degrees, nil
}

func parseWindSpeed(s string) (float64, error) {
	speed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("not float: %q", s)
	}

	if speed < 0 {
		return 0, fmt.Errorf("invalid wind speed: %q", s)
	}

	return speed, nil
}

func scrapeFirstDayWindStates(n *html.Node) ([]string, error) {
	statesNode, ok := htmlutil.Find(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWindState),
	)
	if !ok {
		return nil, errors.New("could not find wind states node")
	}

	var states []string
	if err := htmlutil.ForEach(statesNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			state, err := scrapeWindState(n)
			if err != nil {
				return fmt.Errorf("could not scrape wind state: %w", err)
			}

			states = append(states, state)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				return htmlutil.ErrForEachStopped
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not scrape wind states: %w", err)
	}

	return states, nil
}

func scrapeWindState(n *html.Node) (string, error) {
	var ss []string
	htmlutil.ForEach(n, func(n *html.Node) error {
		if n.Type == html.TextNode {
			ss = append(ss, n.Data)
		}
		return nil
	})

	state := strings.Join(ss, "")
	if state == "" {
		return "", errors.New("invalid wind state")
	}

	return state, nil
}
