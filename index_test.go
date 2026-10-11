package parkingindex

import (
	"encoding/json"
	"strings"
	"testing"
)

// Expected values come from the embedded JSON itself (decoded generically), never typed by hand.
func generic(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCounts(t *testing.T) {
	m := generic(t)
	ap := m["airports"].([]any)
	ls := m["listings"].([]any)
	if got := len(Airports()); got != len(ap) {
		t.Fatalf("Airports() = %d, JSON has %d", got, len(ap))
	}
	if int(m["count"].(float64)) != len(ls) {
		t.Fatalf("count %v != listings %d", m["count"], len(ls))
	}
	total := 0
	for _, a := range Airports() {
		ops := Operators(a.IATA)
		if len(ops) != a.Parkings {
			t.Errorf("%s: Operators() = %d, parkings = %d", a.IATA, len(ops), a.Parkings)
		}
		total += len(ops)
	}
	if total != len(ls) {
		t.Errorf("sum of Operators() = %d, listings = %d", total, len(ls))
	}
	if GeneratedAt() != m["generated_at"] {
		t.Errorf("GeneratedAt() = %q", GeneratedAt())
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	first := Airports()[0]
	a, ok := AirportByIATA(" " + strings.ToLower(first.IATA) + " ")
	if !ok || a.IATA != first.IATA {
		t.Fatalf("lower-case lookup of %s failed", first.IATA)
	}
	if _, ok := AirportByIATA("???"); ok {
		t.Fatal("unknown code found")
	}
	if len(Operators("???")) != 0 || len(PriceStats("???")) != 0 {
		t.Fatal("unknown code returned data")
	}
}

func TestOperatorsSortedByDistance(t *testing.T) {
	for _, a := range Airports() {
		ops := Operators(a.IATA)
		seenNil := false
		for i, l := range ops {
			if l.DistanceKm == nil {
				seenNil = true
				continue
			}
			if seenNil {
				t.Fatalf("%s: listing with distance after one without", a.IATA)
			}
			if i > 0 && ops[i-1].DistanceKm != nil && *ops[i-1].DistanceKm > *l.DistanceKm {
				t.Fatalf("%s: not sorted by distance", a.IATA)
			}
		}
	}
}

// PriceStats must reproduce the aggregates published in airports[].prices (single-currency airports) and must never
// return an aggregate built on fewer than MinPrices prices.
func TestPriceStatsMatchPublishedAggregates(t *testing.T) {
	checked := 0
	for _, a := range Airports() {
		stats := PriceStats(a.IATA)
		currencies := map[string]bool{}
		for _, l := range Operators(a.IATA) {
			if l.Price.Currency != nil {
				currencies[*l.Price.Currency] = true
			}
		}
		byDur := map[Duration]Stat{}
		for _, s := range stats {
			if s.N < MinPrices {
				t.Fatalf("%s %s: aggregate on %d prices", a.IATA, s.Duration, s.N)
			}
			if !(s.Min <= s.Median && s.Median <= s.Max) {
				t.Fatalf("%s %s: min/median/max out of order", a.IATA, s.Duration)
			}
			byDur[s.Duration] = s
		}
		if len(currencies) > 1 {
			continue
		}
		for _, d := range Durations {
			pub := a.Prices[string(d)]
			s, ok := byDur[d]
			if (pub != nil) != ok {
				t.Fatalf("%s %s: published=%v computed=%v", a.IATA, d, pub != nil, ok)
			}
			if pub == nil {
				continue
			}
			if s.Min != pub.Min || s.Median != pub.Median || s.Max != pub.Max || s.N != pub.N {
				t.Fatalf("%s %s: computed %+v, published %+v", a.IATA, d, s, *pub)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no published aggregate was checked")
	}
}
