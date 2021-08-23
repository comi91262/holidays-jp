// downloader for syukujitsu.csv
// https://www8.cao.go.jp/chosei/shukujitsu/gaiyou.html

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// 内閣府ホーム  >  内閣府の政策  >  制度  >  国民の祝日について
// https://www8.cao.go.jp/chosei/shukujitsu/gaiyou.html
const syukujitsuURL = "https://www8.cao.go.jp/chosei/shukujitsu/syukujitsu.csv"

const rawDataPath = "syukujitsu.csv"

func main() {
	if err := _main(); err != nil {
		log.Fatal(err)
	}
}

func _main() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rawData, err := download(ctx)
	if err != nil {
		return err
	}
	if err := formatHolidays(rawData); err != nil {
		return err
	}
	return nil
}

func download(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, syukujitsuURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "https://github.com/shogo82148/holidays-jp")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue to download
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// save the raw data
	if err := os.WriteFile(rawDataPath, buf, 0644); err != nil {
		return nil, err
	}

	return buf, nil
}

func formatHolidays(rawData []byte) error {
	type Holiday struct {
		Date string
		Name string
	}

	reader := transform.NewReader(bytes.NewReader(rawData), japanese.ShiftJIS.NewDecoder())
	csvReader := csv.NewReader(reader)

	// skip 国民の祝日・休日月日,国民の祝日・休日名称 line
	csvReader.Read()

	holidays := []Holiday{}
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		holidays = append(holidays, Holiday{
			Date: formatDate(record[0]),
			Name: record[1],
		})
	}
	sort.Slice(holidays, func(i, j int) bool {
		return holidays[i].Date < holidays[j].Date
	})

	var buf bytes.Buffer
	fmt.Fprint(
		&buf,
		`// Code generated by internal/gen/gen.go; DO NOT EDIT.

		package holiday

		// the year range of pre-calculated holidays
		const (
			holidaysStartYear = `+strings.Split(holidays[0].Date, "-")[0]+`
			holidaysEndYear = `+strings.Split(holidays[len(holidays)-1].Date, "-")[0]+`
		)

		// 内閣府ホーム  >  内閣府の政策  >  制度  >  国民の祝日について
		// https://www8.cao.go.jp/chosei/shukujitsu/gaiyou.html
		// Based on `+syukujitsuURL+`
		var holidays = []Holiday{
		`,
	)
	for _, holiday := range holidays {
		fmt.Fprintf(&buf, "{\nDate: %q,\nName: %q,\n},\n", holiday.Date, holiday.Name)
	}
	fmt.Fprintln(&buf, "}")

	res, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join("holidays-api", "holiday", "holidays_generated.go"), res, 0644)
}

// 2021/1/1 -> 2021-01-01
func formatDate(s string) string {
	date := strings.Split(s, "/")
	y, err := strconv.Atoi(date[0])
	if err != nil {
		panic(err)
	}
	m, err := strconv.Atoi(date[1])
	if err != nil {
		panic(err)
	}
	d, err := strconv.Atoi(date[2])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}
