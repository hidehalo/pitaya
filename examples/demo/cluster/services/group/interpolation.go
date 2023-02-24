package group

import (
	"math"
)

func Linear(y1, y2, mu float64) float64 {
	return y1*(float64(1)-mu) + y2*mu
}

func Cosine(y1, y2, mu float64) float64 {
	mu2 := (float64(1) - math.Cos(mu*math.Pi)) / float64(2)
	return (y1*(1-mu2) + y2*mu2)
}

func NearestNeighbour() {

}

func CubicSpline(y0, y1, y2, y3, mu float64) float64 {
	mu2 := mu * mu
	a0 := y3 - y2 - y0 + y1
	a1 := y0 - y1 - a0
	a2 := y2 - y0
	a3 := y1
	return (a0*mu*mu2 + a1*mu2 + a2*mu + a3)
}

func ShapePreservation() {

}

func ThinPlateSpline() {

}

func Biharmonic() {

}

func Quadratic() {

}
