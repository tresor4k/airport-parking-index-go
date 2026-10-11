# airport-parking-index-go

Go package embedding the [Park & Keep airport parking index](https://parkandkeep.com/en/data/airport-parking-index)
(page en français : https://parkandkeep.com/donnees/indice-parkings-aeroport): car parks serving European
airports, their attributes and public prices for 3, 7 and 14-day stays read on each operator's own website.

```go
import parkingindex "github.com/tresor4k/airport-parking-index-go"

a, ok := parkingindex.AirportByIATA("bru")      // airport + published aggregates
ops := parkingindex.Operators("BRU")             // car parks, nearest first
for _, s := range parkingindex.PriceStats("BRU") {
	fmt.Println(s.Duration, s.Currency, s.Min, s.Median, s.Max, s.N) // only from 3 prices
}
```

No network access: the snapshot is compiled in (`go:embed`). `GeneratedAt()` gives its date; the module is re-tagged
when a new monthly snapshot is published. The dataset, its method and field dictionary:
https://github.com/tresor4k/airport-parking-index

Licences: code MIT ([LICENSE](LICENSE)); embedded data CC BY 4.0 — credit "Park & Keep" with a link to the data page.
