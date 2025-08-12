//go:build cgo
package external

import (
	"strings"

	"github.com/openvenues/gopostal/expand"
	"github.com/openvenues/gopostal/parser"
)

type LP struct {
	House, Road, Unit, Level, Ward, City, Province string
	Coverage                                        float64
}

func ExtractWithLibpostal(raw string) LP {
	opts := expand.DefaultOptions()
	opts.Languages = []string{"vi"}
	exps := expand.ExpandAddress(raw, opts)
	best := raw
	if len(exps) > 0 {
		best = exps[0]
	}
	comps := parser.ParseAddress(best)
	covered, total := 0, len(strings.Fields(best))
	lp := LP{}
	for _, c := range comps {
		switch c.Label {
		case "house_number":
			lp.House = c.Value
		case "road":
			lp.Road = c.Value
		case "unit":
			lp.Unit = c.Value
		case "level":
			lp.Level = c.Value
		case "suburb":
			lp.Ward = c.Value
		case "city":
			lp.City = c.Value
		case "state":
			lp.Province = c.Value
		}
		covered += len(strings.Fields(c.Value))
	}
	if total > 0 {
		lp.Coverage = float64(covered) / float64(total)
	}
	return lp
}
