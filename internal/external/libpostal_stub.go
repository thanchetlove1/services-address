//go:build !cgo
package external

type LP struct {
	House, Road, Unit, Level, Ward, City, Province string
	Coverage                                        float64
}

func ExtractWithLibpostal(raw string) LP { return LP{} }