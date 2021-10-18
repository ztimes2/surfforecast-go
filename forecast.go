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
	pathFormatForecastsForEightDays = "/breaks/%s/forecasts/latest"
)

const (
	classBreakHeaderIssued   = "break-header__issued"
	classForecastTableBasic  = "forecast-table__basic"
	classForecastTableRow    = "forecast-table__row"
	classForecastTableCell   = "forecast-table__cell"
	classForecastTableValue  = "forecast-table__value"
	classForecastTableTime   = "forecast-table-time"
	classForecastTableDays   = "forecast-table-days"
	classForecastTableRating = "forecast-table-rating"
	classIsDayEnd            = "is-day-end"
	classWindIcon            = "wind-icon"
	classWindLetters         = "wind-icon__letters"
	classWindIconArrow       = "wind-icon__arrow"

	attributeDataRowName    = "data-row-name"
	attributeDataSwellState = "data-swell-state"
	attributeDataSpeed      = "data-speed"

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

func (s *Scraper) ForecastsForEightDays(breakName string) (*Forecasts, error) {
	// TODO enable context propogation and cancelation
	// TODO use chromedp to dynamically expand first day's forecast

	path := fmt.Sprintf(pathFormatForecastsForEightDays, breakName)

	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrBreakNotFound
		}
		return nil, fmt.Errorf("received response with %d status code", resp.StatusCode)
	}

	defer resp.Body.Close()
	node, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not parse response body as html: %w", err)
	}

	forecasts, err := scrapeForecasts(node, s.timezones)
	if err != nil {
		return nil, fmt.Errorf("could not scrape html: %w", err)
	}

	return forecasts, nil
}

type Forecasts struct {
	IssuedAt time.Time
	Daily    []*DailyForecast
}

func newForecasts(
	issuedAt time.Time,
	days []int,
	hours [][]int,
	ratings [][]int,
	swells [][]Swells,
	waveEnergies [][]float64,
	winds [][]wind,
	windStates [][]string) (*Forecasts, error) {

	if len(days) != len(hours) {
		return nil, errors.New("days and hours must have equal number of elements")
	}
	if len(days) != len(ratings) {
		return nil, errors.New("days and ratings must have equal number of elements")
	}
	if len(days) != len(swells) {
		return nil, errors.New("days and swells must have equal number of elements")
	}
	if len(days) != len(waveEnergies) {
		return nil, errors.New("days and wave energies must have equal number of elements")
	}
	if len(days) != len(winds) {
		return nil, errors.New("days and winds must have equal number of elements")
	}
	if len(days) != len(windStates) {
		return nil, errors.New("days and wind states must have equal number of elements")
	}

	var (
		forecasts = make([]*DailyForecast, len(days))
		year      = issuedAt.Year()
		month     = issuedAt.Month()

		previous *DailyForecast
	)
	for i := range forecasts {
		if previous != nil {
			if previous.Timestamp.Day() > days[i] {
				if month+1 > time.December {
					month = time.January
				}
				month++
			}

			if previous.Timestamp.Month() > month {
				year++
			}
		}

		f, err := newDailyForecast(
			issuedAt.Location(),
			issuedAt.Year(),
			month,
			days[i],
			hours[i],
			ratings[i],
			swells[i],
			waveEnergies[i],
			winds[i],
			windStates[i],
		)
		if err != nil {
			return nil, fmt.Errorf("could not create forecast: %w", err)
		}

		forecasts[i] = f
		previous = f
	}

	return &Forecasts{
		IssuedAt: issuedAt,
		Daily:    forecasts,
	}, nil
}

type DailyForecast struct {
	Timestamp time.Time
	Hourly    []HourlyForecast
}

func newDailyForecast(
	l *time.Location,
	year int,
	month time.Month,
	day int,
	hours []int,
	ratings []int,
	swells []Swells,
	waveEnergies []float64,
	winds []wind,
	windStates []string) (*DailyForecast, error) {

	if len(hours) != len(ratings) {
		return nil, errors.New("hours and ratings must have equal number of elements")
	}
	if len(hours) != len(swells) {
		return nil, errors.New("hours and swells must have equal number of elements")
	}
	if len(hours) != len(waveEnergies) {
		return nil, errors.New("hours and wave energies must have equal number of elements")
	}
	if len(hours) != len(winds) {
		return nil, errors.New("hours and winds must have equal number of elements")
	}
	if len(hours) != len(windStates) {
		return nil, errors.New("hours and wind states must have equal number of elements")
	}

	forecasts := make([]HourlyForecast, len(hours))
	for i := range forecasts {
		forecasts[i].Timestamp = time.Date(year, month, day, hours[i], 0, 0, 0, l)
		forecasts[i].Rating = ratings[i]
		forecasts[i].Swells = swells[i]
		forecasts[i].WaveEnergyInKiloJoules = waveEnergies[i]
		forecasts[i].Wind = Wind{
			SpeedInKilometersPerHour:     winds[i].speed,
			DirectionToInDegrees:         winds[i].degrees,
			DirectionFromInCompassPoints: winds[i].letters,
			State:                        windStates[i],
		}
	}

	return &DailyForecast{
		Timestamp: time.Date(year, month, day, 0, 0, 0, 0, l),
		Hourly:    forecasts,
	}, nil
}

type HourlyForecast struct {
	Timestamp              time.Time
	Rating                 int
	Swells                 Swells
	WaveEnergyInKiloJoules float64
	Wind                   Wind
	// TODO tide
}

type Swells []Swell

type Swell struct {
	PeriodInSeconds              float64
	DirectionToInDegrees         float64
	DirectionFromInCompassPoints string
	WaveHeightInMeters           float64
}

type Wind struct {
	SpeedInKilometersPerHour     float64
	DirectionToInDegrees         float64
	DirectionFromInCompassPoints string
	State                        string
}

func scrapeForecasts(n *html.Node, tz *timezone.Timezone) (*Forecasts, error) {
	issuedAt, err := scrapeIssueTimestamp(n, tz)
	if err != nil {
		return nil, fmt.Errorf("could not scrape issue date: %w", err)
	}

	tableNode, ok := htmlutil.FindOne(n, htmlutil.WithClassEqual(classForecastTableBasic))
	if !ok {
		return nil, errors.New("could not find table node")
	}

	days, err := scrapeDays(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape days: %w", err)
	}

	hours, err := scrapeHours(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape hours: %w", err)
	}

	ratings, err := scrapeRatings(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape ratings: %w", err)
	}

	swells, err := scrapeSwells(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape swells: %w", err)
	}

	waveEnergies, err := scrapeWaveEnergies(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape wave energies: %w", err)
	}

	winds, err := scrapeWinds(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape winds: %w", err)
	}

	windStates, err := scrapeWindStates(tableNode)
	if err != nil {
		return nil, fmt.Errorf("could not scrape wind states: %w", err)
	}

	return newForecasts(
		issuedAt,
		days,
		hours,
		ratings,
		swells,
		waveEnergies,
		winds,
		windStates,
	)
}

func scrapeIssueTimestamp(n *html.Node, tz *timezone.Timezone) (time.Time, error) {
	issueNode, ok := htmlutil.FindOne(n, htmlutil.WithClassEqual(classBreakHeaderIssued))
	if !ok {
		return time.Time{}, errors.New("could not find issue node")
	}

	issueTextNode := issueNode.FirstChild
	if issueTextNode == nil {
		return time.Time{}, errors.New("could not find issue text node")
	}

	parts := strings.Split(issueTextNode.Data, " ")
	if len(parts) != 12 {
		return time.Time{}, fmt.Errorf("unexpected issue text: %q", issueTextNode.Data)
	}

	hourText := parts[5]
	clockPeriodText := parts[6]
	dayText := parts[8]
	monthText := parts[9]
	yearText := parts[10]
	tzAbbr := parts[11]

	hour, err := parseTwelveClockHour(hourText)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse issue hour: %w", err)
	}

	clockPeriod, err := parseClockPeriod(clockPeriodText)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse clock period: %w", err)
	}

	hour = toTwentyFourClockHour(hour, clockPeriod)

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

	return time.Date(year, month, day, hour, 0, 0, 0, loc), nil
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

func scrapeDays(n *html.Node) ([]int, error) {
	daysNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableDays),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameDays),
	)
	if !ok {
		return nil, errors.New("could not find days node")
	}

	var days []int
	if err := htmlutil.ForEach(daysNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			day, err := scrapeDay(n)
			if err != nil {
				return fmt.Errorf("could not scrape day: %w", err)
			}

			days = append(days, day)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return days, nil
}

func scrapeDay(n *html.Node) (int, error) {
	nodes := htmlutil.Find(n, htmlutil.WithClassEqual(classForecastTableValue))
	if len(nodes) != 2 {
		return 0, errors.New("unexpected table values")
	}

	dayTextNode := nodes[1].FirstChild
	if dayTextNode == nil {
		return 0, errors.New("could not find day text node")
	}

	day, err := parseDay(dayTextNode.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse day: %w", err)
	}

	return day, nil
}

func scrapeHours(n *html.Node) ([][]int, error) {
	hoursNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableTime),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameTime),
	)
	if !ok {
		return nil, errors.New("could not find hours node")
	}

	var (
		allHours [][]int
		hours    []int
	)
	if err := htmlutil.ForEach(hoursNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			hour, err := scrapeHour(n)
			if err != nil {
				return fmt.Errorf("could not scrape hour: %w", err)
			}

			hours = append(hours, hour)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				allHours = append(allHours, hours)
				hours = []int{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allHours, nil
}

func scrapeHour(n *html.Node) (int, error) {
	nodes := htmlutil.Find(n, htmlutil.WithClassEqual(classForecastTableValue))
	if len(nodes) != 2 {
		return 0, errors.New("unexpected table values")
	}

	hourTextNode := nodes[0].FirstChild
	if hourTextNode == nil {
		return 0, errors.New("could not find hour text node")
	}

	hour, err := parseTwelveClockHour(hourTextNode.Data)
	if err != nil {
		return 0, fmt.Errorf("could not parse hour: %w", err)
	}

	periodTextNode := nodes[1].FirstChild
	if periodTextNode == nil {
		return 0, errors.New("could not find clock period text node")
	}

	period, err := parseClockPeriod(periodTextNode.Data)
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
	switch strings.ToUpper(s) {
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

func scrapeRatings(n *html.Node) ([][]int, error) {
	ratingsNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassContaining(classForecastTableRow, classForecastTableRating),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameRating),
	)
	if !ok {
		return nil, errors.New("could not find ratings node")
	}

	var (
		allRatings [][]int
		ratings    []int
	)
	if err := htmlutil.ForEach(ratingsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			ratingAttr, ok := htmlutil.Attribute(n.FirstChild, htmlutil.AttributeAlternateImageText)
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
				allRatings = append(allRatings, ratings)
				ratings = []int{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allRatings, nil
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

func scrapeSwells(n *html.Node) ([][]Swells, error) {
	swellsNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWaveHeight),
	)
	if !ok {
		return nil, errors.New("could not find swells node")
	}

	var (
		allSwells [][]Swells
		swells    []Swells
	)
	if err := htmlutil.ForEach(swellsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			hourlySwells, err := scrapeHourlySwells(n)
			if err != nil {
				return fmt.Errorf("could not scrape hourly swells: %w", err)
			}

			swells = append(swells, hourlySwells)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				allSwells = append(allSwells, swells)
				swells = []Swells{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allSwells, nil
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
			PeriodInSeconds:              p.Period,
			DirectionToInDegrees:         p.Angle,
			DirectionFromInCompassPoints: p.Letters,
			WaveHeightInMeters:           p.Height,
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

func scrapeWaveEnergies(n *html.Node) ([][]float64, error) {
	energiesNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameEnergy),
	)
	if !ok {
		return nil, errors.New("could not find wave energies node")
	}

	var (
		allEnergies [][]float64
		energies    []float64
	)
	if err := htmlutil.ForEach(energiesNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			energy, err := scrapeWaveEnergy(n)
			if err != nil {
				return fmt.Errorf("could not scrape wave energy: %w", err)
			}

			energies = append(energies, energy)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				allEnergies = append(allEnergies, energies)
				energies = []float64{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allEnergies, nil
}

func scrapeWaveEnergy(n *html.Node) (float64, error) {
	energyNode := n.FirstChild
	if energyNode == nil {
		return 0, errors.New("could not find wave energy node")
	}

	energyTextNode := energyNode.FirstChild
	if energyTextNode == nil {
		return 0, errors.New("could not find wave energy text node")
	}

	energy, err := parseWaveEnergy(energyTextNode.Data)
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

func scrapeWinds(n *html.Node) ([][]wind, error) {
	windsNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWind),
	)
	if !ok {
		return nil, errors.New("could not find winds node")
	}

	var (
		allWinds [][]wind
		winds    []wind
	)
	if err := htmlutil.ForEach(windsNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			w, err := scrapeWind(n)
			if err != nil {
				return fmt.Errorf("could not scrape wind: %w", err)
			}

			winds = append(winds, w)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				allWinds = append(allWinds, winds)
				winds = []wind{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allWinds, nil
}

func scrapeWind(n *html.Node) (wind, error) {
	iconNode, ok := htmlutil.FindOne(n, htmlutil.WithClassEqual(classWindIcon))
	if !ok {
		return wind{}, errors.New("could not find wind icon node")
	}

	speedAttr, ok := htmlutil.Attribute(iconNode, attributeDataSpeed)
	if !ok {
		return wind{}, errors.New("could not find wind speed attribute")
	}

	speed, err := parseWindSpeed(speedAttr.Val)
	if err != nil {
		return wind{}, fmt.Errorf("could not parse wind speed: %w", err)
	}

	degrees, err := scrapeWindDirectionDegrees(iconNode)
	if err != nil {
		return wind{}, fmt.Errorf("could not scrape wind direction degrees: %w", err)
	}

	lettersNode, ok := htmlutil.FindOne(iconNode, htmlutil.WithClassEqual(classWindLetters))
	if !ok {
		return wind{}, errors.New("could not find wind direction letters node")
	}

	lettersTextNode := lettersNode.FirstChild
	if lettersTextNode == nil {
		return wind{}, errors.New("could not find wind direction letters text node")
	}

	return wind{
		speed:   speed,
		degrees: degrees,
		letters: lettersTextNode.Data,
	}, nil
}

type wind struct {
	speed   float64
	degrees float64
	letters string
}

func scrapeWindDirectionDegrees(n *html.Node) (float64, error) {
	arrowNode, ok := htmlutil.FindOne(n, htmlutil.WithClassEqual(classWindIconArrow))
	if !ok {
		return 0, errors.New("could not find wind direction arrow node")
	}

	attr, ok := htmlutil.Attribute(arrowNode, htmlutil.AttributeTransform)
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

func scrapeWindStates(n *html.Node) ([][]string, error) {
	statesNode, ok := htmlutil.FindOne(
		n,
		htmlutil.WithClassEqual(classForecastTableRow),
		htmlutil.WithAttributeEqual(attributeDataRowName, dataRowNameWindState),
	)
	if !ok {
		return nil, errors.New("could not find wind states node")
	}

	var (
		allStates [][]string
		states    []string
	)
	if err := htmlutil.ForEach(statesNode, func(n *html.Node) error {
		if htmlutil.ClassContains(n, classForecastTableCell) {
			state, err := scrapeWindState(n)
			if err != nil {
				return fmt.Errorf("could not scrape wind state: %w", err)
			}

			states = append(states, state)

			isDayEnd := htmlutil.ClassContains(n, classIsDayEnd)
			if isDayEnd {
				allStates = append(allStates, states)
				states = []string{}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return allStates, nil
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
