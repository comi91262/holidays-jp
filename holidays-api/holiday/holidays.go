package holiday

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Holiday struct {
	Date string
	Name string
}

type withDate []Holiday

func (s withDate) Len() int           { return len(s) }
func (s withDate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s withDate) Less(i, j int) bool { return s[i].Date < s[j].Date }

// findHoliday returns whether the specific day is a holiday.
func findHoliday(year int, month time.Month, day int) (Holiday, bool) {
	date := fmt.Sprintf("%04d-%02d-%02d", year, int(month), day)
	idx := sort.Search(len(holidays), func(i int) bool {
		return holidays[i].Date >= date
	})

	if idx < len(holidays) && holidays[idx].Date == date {
		return holidays[idx], true
	}
	return Holiday{}, false
}

// findHolidaysInMonth returns holidays in the specific month.
func findHolidaysInMonth(year int, month time.Month) []Holiday {
	startDate := fmt.Sprintf("%04d-%02d-01", year, int(month))
	endDate := fmt.Sprintf("%04d-%02d-99", year, int(month))

	start := sort.Search(len(holidays), func(i int) bool {
		return holidays[i].Date >= startDate
	})
	end := sort.Search(len(holidays), func(i int) bool {
		return holidays[i].Date >= endDate
	})
	return holidays[start:end]
}

// findHolidaysInYear returns holidays in the specific year.
func findHolidaysInYear(year int) []Holiday {
	startDate := fmt.Sprintf("%04d-01-01", year)
	endDate := fmt.Sprintf("%04d-99-99", year)

	start := sort.Search(len(holidays), func(i int) bool {
		return holidays[i].Date >= startDate
	})
	end := sort.Search(len(holidays), func(i int) bool {
		return holidays[i].Date >= endDate
	})
	return holidays[start:end]
}

type annuallyHolidaysRule struct {
	// BeginYear is a year that the law is enforced
	BeginYear int

	// StaticHolydays are holydays that are on the same date every year
	StaticHolydays []staticHolyday

	// StaticHolydays are holydays that are on the same weekday in the month.
	WeekdayHolydays []weekdayHolyday
}

type staticHolyday struct {
	Date string // MM-DD
	Name string
}

type weekdayHolyday struct {
	Month   time.Month
	Weekday time.Weekday
	Index   int
	Name    string
}

func calcHolidaysInMonthWithoutInLieu(year int, month time.Month) []Holiday {
	// search the rule of this year
	var rule *annuallyHolidaysRule
	for i := len(annuallyHolidaysRules); i > 0; i-- {
		if annuallyHolidaysRules[i-1].BeginYear >= year {
			rule = &annuallyHolidaysRules[i-1]
			break
		}
	}
	if rule == nil {
		return nil
	}

	var holydays []Holiday
	yearPrefix := fmt.Sprintf("%04d-", year)
	monthPrefix := fmt.Sprintf("%02d-", int(month))
	for _, d := range rule.StaticHolydays {
		if strings.HasPrefix(d.Date, monthPrefix) {
			holydays = append(holydays, Holiday{
				Date: yearPrefix + d.Date,
				Name: d.Name,
			})
		}
	}

	weekdayOfFirstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Weekday()
	_ = weekdayOfFirstDay
	for _, d := range rule.WeekdayHolydays {
		if d.Month == month {
			day := int(d.Weekday - weekdayOfFirstDay)
			if day < 0 {
				day += 7
			}
			day += d.Index*7 + 1
			holydays = append(holydays, Holiday{
				Date: fmt.Sprintf("%04d-%02d-%02d", year, int(month), day),
				Name: d.Name,
			})
		}
	}

	// Vernal Equinox Day
	if month == time.March {
		holydays = append(holydays, Holiday{
			Date: fmt.Sprintf("%04d-%02d-%02d", year, int(month), vernalEquinoxDay(year)),
			Name: "春分の日",
		})
	}

	// Autumnal Equinox Day
	if month == time.September {
		holydays = append(holydays, Holiday{
			Date: fmt.Sprintf("%04d-%02d-%02d", year, int(month), autumnalEquinoxDay(year)),
			Name: "秋分の日",
		})
	}

	yearMonthPrefix := yearPrefix + monthPrefix
	for _, d := range specialHolidays {
		if strings.HasPrefix(d.Date, yearMonthPrefix) {
			holydays = append(holydays, d)
		}
	}

	sort.Sort(withDate(holydays))
	return holydays
}

func calcHolidaysInMonth(year int, month time.Month) []Holiday {
	// add holidays in lieu
	return calcHolidaysInMonthWithoutInLieu(year, month)
}

func calcHolidaysInYear(year int) []Holiday {
	var result []Holiday
	for month := time.January; month <= time.December; month++ {
		holidays := calcHolidaysInMonth(year, month)
		result = append(result, holidays...)
	}
	return result
}

// from 長沢 工(1999) "日の出・日の入りの計算 天体の出没時刻の求め方" 株式会社地人書館
var sunLongitudeTable = [...][3]float64{
	{0.0200, 355.05, 719.981},
	{0.0048, 234.95, 19.341},
	{0.0020, 247.1, 329.64},
	{0.0018, 297.8, 4452.67},
	{0.0018, 251.3, 0.20},
	{0.0015, 343.2, 450.37},
	{0.0013, 81.4, 225.18},
	{0.0008, 132.5, 659.29},
	{0.0007, 153.3, 90.38},
	{0.0007, 206.8, 30.35},
	{0.0006, 29.8, 337.18},
	{0.0005, 207.4, 1.50},
	{0.0005, 291.2, 22.81},
	{0.0004, 234.9, 315.56},
	{0.0004, 157.3, 299.30},
	{0.0004, 21.1, 720.02},
	{0.0003, 352.5, 1079.97},
	{0.0003, 329.7, 44.43},
}

// julianYear is a number of julian years from J2000.0(2000/01/01 12:00 Terrestrial Time)
type julianYear float64

var j2000 = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC).Unix()

func time2JulianYear(t time.Time) julianYear {
	d := t.Unix() - j2000

	// convert UTC(Coordinated Universal Time) into TAI(International Atomic Time)
	d += 36 // TAI - UTC = 36seconds (at 2015/08)

	// convert TAI into TT(Terrestrial Time)
	d += 32
	return julianYear(float64(d) / ((365*24 + 6) * 60 * 60))
}

func sunLongitude(jy julianYear) float64 {
	t := float64(jy)
	l := normalizeDegree(360.00769 * t)
	l = normalizeDegree(l + 280.4603)
	l = normalizeDegree(l + (1.9146-0.00005*t)*sin(357.538+359.991*t))
	for _, b := range sunLongitudeTable {
		l = normalizeDegree(l + b[0]*sin(b[1]+b[2]*t))
	}
	return l
}

func sin(x float64) float64 {
	return math.Sin(x / 180 * math.Pi)
}

func normalizeDegree(x float64) float64 {
	x = math.Mod(x, 360)
	if x < 0 {
		x += 360
	}
	return x
}

var jst *time.Location

func init() {
	var err error
	jst, err = time.LoadLocation("Asia/Tokyo")
	if err != nil {
		panic(err)
	}
}

func vernalEquinoxDay(year int) int {
	for i := 10; i <= 31; i++ {
		t := time.Date(year, time.March, i, 0, 0, 0, 0, jst)
		l := sunLongitude(time2JulianYear(t))
		if l < 180 {
			return i - 1
		}
	}
	return 0
}

func autumnalEquinoxDay(year int) int {
	for i := 10; i <= 30; i++ {
		t := time.Date(year, time.September, i, 0, 0, 0, 0, jst)
		l := sunLongitude(time2JulianYear(t))
		if l >= 180 {
			return i - 1
		}
	}
	return 0
}
