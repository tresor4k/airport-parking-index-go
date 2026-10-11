// Package parkingindex embeds the Park & Keep airport parking index (Europe): car parks serving
// European airports, their attributes (shuttle, valet, covered, distance) and public prices for 3, 7 and
// 14-day stays read on each operator's own website, never on an aggregator.
//
// Data page: https://parkandkeep.com/en/data/airport-parking-index
// (français : https://parkandkeep.com/donnees/indice-parkings-aeroport).
// The embedded data is licensed CC BY 4.0: credit "Park & Keep" with a link to the data page.
// The Go code is MIT licensed.
//
// Typical use:
//
//	a, ok := parkingindex.AirportByIATA("BRU")
//	ops := parkingindex.Operators("BRU")
//	stats := parkingindex.PriceStats("BRU") // min / median / max per duration, only from 3 prices
package parkingindex

import (
	_ "embed"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"sync"
)

//go:embed data/index.json
var raw []byte

// DataPageURL is the public page of the index (English).
const DataPageURL = "https://parkandkeep.com/en/data/airport-parking-index"

// DataPageURLFr is the public page of the index (French).
const DataPageURLFr = "https://parkandkeep.com/donnees/indice-parkings-aeroport"

// MinPrices is the minimum number of prices for an aggregate to be computed.
const MinPrices = 3

// Duration is a stay length of the index: D3, D7 or D14.
type Duration string

// Stay lengths priced by the index.
const (
	D3  Duration = "d3"
	D7  Duration = "d7"
	D14 Duration = "d14"
)

// Durations lists the stay lengths in increasing order.
var Durations = []Duration{D3, D7, D14}

// AirportRef identifies the airport a listing serves.
type AirportRef struct {
	IATA     string `json:"iata"`
	City     string `json:"city"`
	Country  string `json:"country"`
	Vertical string `json:"vertical"`
}

// Attributes are the services of a car park; nil means not stated by the operator.
type Attributes struct {
	Shuttle       *bool `json:"shuttle"`
	Valet         *bool `json:"valet"`
	Covered       *bool `json:"covered"`
	Open24h       *bool `json:"open_24h"`
	KeysKept      *bool `json:"keys_kept"`
	EVCharging    *bool `json:"ev_charging"`
	OnlineBooking *bool `json:"online_booking"`
	OnlinePayment *bool `json:"online_payment"`
}

// Price holds the public prices of a listing; nil amounts were not collected (never free).
type Price struct {
	D3        *float64 `json:"d3"`
	D7        *float64 `json:"d7"`
	D14       *float64 `json:"d14"`
	Currency  *string  `json:"currency"`
	SourceURL *string  `json:"source_url"`
	CheckedAt *string  `json:"checked_at"`
	StayStart *string  `json:"stay_start"`
	Method    *string  `json:"method"`
}

// Amount returns the price of the given duration, if collected.
func (p Price) Amount(d Duration) (float64, bool) {
	var v *float64
	switch d {
	case D3:
		v = p.D3
	case D7:
		v = p.D7
	case D14:
		v = p.D14
	}
	if v == nil {
		return 0, false
	}
	return *v, true
}

// Listing is one car park serving an airport.
type Listing struct {
	ID           string     `json:"id"`
	Airport      AirportRef `json:"airport"`
	Name         string     `json:"name"`
	City         *string    `json:"city"`
	PostalCode   *string    `json:"postal_code"`
	Lat          *float64   `json:"lat"`
	Lng          *float64   `json:"lng"`
	DistanceKm   *float64   `json:"distance_km"`
	Attributes   Attributes `json:"attributes"`
	Website      *string    `json:"website"`
	LastVerified string     `json:"last_verified"`
	Price        Price      `json:"price"`
}

// Aggregate is the min / median / max of one duration at one airport, as published in the index.
type Aggregate struct {
	Min    float64 `json:"min"`
	Median float64 `json:"median"`
	Max    float64 `json:"max"`
	N      int     `json:"n"`
}

// Airport is one airport with the aggregates the index publishes for it.
type Airport struct {
	IATA             string                `json:"iata"`
	City             string                `json:"city"`
	Country          string                `json:"country"`
	Vertical         string                `json:"vertical"`
	Parkings         int                   `json:"parkings"`
	ShareShuttle     float64               `json:"share_shuttle"`
	ShareValet       float64               `json:"share_valet"`
	ShareCovered     float64               `json:"share_covered"`
	DistanceKmMedian *float64              `json:"distance_km_median"`
	DistanceKmMin    *float64              `json:"distance_km_min"`
	PricesCollected  int                   `json:"prices_collected"`
	Prices           map[string]*Aggregate `json:"prices"`
}

// Index is the whole snapshot.
type Index struct {
	GeneratedAt string    `json:"generated_at"`
	License     string    `json:"license"`
	Attribution string    `json:"attribution"`
	MethodURL   string    `json:"method_url"`
	Count       int       `json:"count"`
	Airports    []Airport `json:"airports"`
	Listings    []Listing `json:"listings"`
}

var (
	once sync.Once
	idx  *Index
	err  error
)

// Load parses the embedded snapshot once and returns it. The returned value is shared: do not modify it.
func Load() (*Index, error) {
	once.Do(func() {
		var v Index
		if err = json.Unmarshal(raw, &v); err == nil {
			idx = &v
		}
	})
	return idx, err
}

func mustLoad() *Index {
	v, e := Load()
	if e != nil {
		panic("parkingindex: embedded data is invalid: " + e.Error())
	}
	return v
}

// GeneratedAt returns the generation timestamp of the embedded snapshot (RFC 3339).
func GeneratedAt() string { return mustLoad().GeneratedAt }

// Airports returns the airports of the index, in file order.
func Airports() []Airport { return mustLoad().Airports }

// AirportByIATA returns the airport with the given IATA code (case-insensitive).
func AirportByIATA(code string) (Airport, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, a := range mustLoad().Airports {
		if a.IATA == code {
			return a, true
		}
	}
	return Airport{}, false
}

// Operators returns the car parks serving the airport with the given IATA code (case-insensitive), nearest first;
// listings without a known distance come last, then by name.
func Operators(code string) []Listing {
	code = strings.ToUpper(strings.TrimSpace(code))
	var out []Listing
	for _, l := range mustLoad().Listings {
		if l.Airport.IATA == code {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := out[i].DistanceKm, out[j].DistanceKm
		switch {
		case di != nil && dj != nil && *di != *dj:
			return *di < *dj
		case (di == nil) != (dj == nil):
			return di != nil
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Stat is the min / median / max of the prices of one duration at one airport, in one currency.
type Stat struct {
	Duration Duration
	Currency string
	Min      float64
	Median   float64
	Max      float64
	N        int
	// FirstChecked and LastChecked bound the dates the prices were read (YYYY-MM-DD).
	FirstChecked string
	LastChecked  string
}

// PriceStats computes, for the airport with the given IATA code, one Stat per duration and currency that has at
// least MinPrices prices. Durations with fewer prices are left out, so an empty result means "not enough prices"
// rather than "no parking". Values are rounded to 2 decimals like the published aggregates.
func PriceStats(code string) []Stat {
	type key struct {
		d Duration
		c string
	}
	vals := map[key][]float64{}
	dates := map[key][]string{}
	for _, l := range Operators(code) {
		if l.Price.Currency == nil {
			continue
		}
		for _, d := range Durations {
			if v, ok := l.Price.Amount(d); ok {
				k := key{d, *l.Price.Currency}
				vals[k] = append(vals[k], v)
				if l.Price.CheckedAt != nil {
					dates[k] = append(dates[k], (*l.Price.CheckedAt)[:min(10, len(*l.Price.CheckedAt))])
				}
			}
		}
	}
	var out []Stat
	for _, d := range Durations {
		var currencies []string
		for k := range vals {
			if k.d == d {
				currencies = append(currencies, k.c)
			}
		}
		sort.Strings(currencies)
		for _, c := range currencies {
			v := vals[key{d, c}]
			if len(v) < MinPrices {
				continue
			}
			sort.Float64s(v)
			s := Stat{Duration: d, Currency: c, Min: round2(v[0]), Median: round2(median(v)), Max: round2(v[len(v)-1]), N: len(v)}
			if ds := dates[key{d, c}]; len(ds) > 0 {
				sort.Strings(ds)
				s.FirstChecked, s.LastChecked = ds[0], ds[len(ds)-1]
			}
			out = append(out, s)
		}
	}
	return out
}

// median expects sorted values.
func median(v []float64) float64 {
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }
