package geometry

import (
	"math"
)

// LineOfSight returns a list of points that are in the line of sight between the source and the target.
// It will include the source as the first point.
// It will include the last position which did not return true for isTransparent.
// If there is a LoS, the last point will be the target.
func LineOfSight(source Point, target Point, canTraverse func(p Point) bool) []Point {
	var results []Point
	x0, y0 := source.X, source.Y
	x1, y1 := target.X, target.Y
	dx := Abs(x1 - x0)
	var sx, sy int
	if x0 < x1 {
		sx = 1
	} else {
		sx = -1
	}
	if y0 < y1 {
		sy = 1
	} else {
		sy = -1
	}
	dy := -Abs(y1 - y0)
	err := dx + dy
	var e2 int /* error value e_xy */
	for {
		gruidPoint := Point{X: x0, Y: y0}
		pointIsVisible := canTraverse(gruidPoint)
		results = append(results, gruidPoint)
		if (x0 == x1 && y0 == y1) || !pointIsVisible {
			break
		}
		e2 = 2 * err
		if e2 > dy {
			err += dy
			x0 += sx
		} /* e_xy+e_x > 0 */
		if e2 < dx {
			err += dx
			y0 += sy
		} /* e_xy+e_y < 0 */
	}
	return results
}

type CompassDirection float64

func (d CompassDirection) Opposite() CompassDirection {
	d += 180
	if d >= 360 {
		d -= 360
	}
	return d
}

func (d CompassDirection) ToPoint() Point {
	switch d {
	case East:
		return Point{X: 1, Y: 0}
	case SouthEast:
		return Point{X: 1, Y: 1}
	case South:
		return Point{X: 0, Y: 1}
	case SouthWest:
		return Point{X: -1, Y: 1}
	case West:
		return Point{X: -1, Y: 0}
	case NorthWest:
		return Point{X: -1, Y: -1}
	case North:
		return Point{X: 0, Y: -1}
	case NorthEast:
		return Point{X: 1, Y: -1}
	}
	return Point{}
}

func (d CompassDirection) TurnRightBy90() CompassDirection {
	d += 90
	if d >= 360 {
		d -= 360
	}
	return d
}

func (d CompassDirection) TurnLeftBy90() CompassDirection {
	d -= 90
	if d < 0 {
		d += 360
	}
	return d
}

const (
	East      CompassDirection = 0
	SouthEast CompassDirection = 45
	South     CompassDirection = 90
	SouthWest CompassDirection = 135
	West      CompassDirection = 180
	NorthWest CompassDirection = 225
	North     CompassDirection = 270
	NorthEast CompassDirection = 315
)

var OnePointF = PointF{X: 1, Y: 1}

type PointF struct {
	X float64
	Y float64
}

func (f PointF) Mul(scalar float64) PointF {
	return PointF{X: f.X * scalar, Y: f.Y * scalar}
}

func (f PointF) MulInt(scalar int) PointF {
	return f.Mul(float64(scalar))
}
func (f PointF) Normalize() PointF {
	length := math.Sqrt(f.X*f.X + f.Y*f.Y)
	if length == 0 {
		return f
	}
	return PointF{X: f.X / length, Y: f.Y / length}
}

func (f PointF) Rotate(degrees int) PointF {
	radians := float64(degrees) * (math.Pi / 180.0)
	cos := math.Cos(radians)
	sin := math.Sin(radians)
	x := f.X*cos - f.Y*sin
	y := f.X*sin + f.Y*cos
	return PointF{X: x, Y: y}
}

func (f PointF) ToPoint() Point {
	return Point{X: int(f.X), Y: int(f.Y)}
}

func (f PointF) Add(other PointF) PointF {
	return PointF{X: f.X + other.X, Y: f.Y + other.Y}
}

func (f PointF) Div(value float64) PointF {
	return PointF{X: f.X / value, Y: f.Y / value}
}

func (f PointF) Sub(origin PointF) PointF {
	return PointF{X: f.X - origin.X, Y: f.Y - origin.Y}
}
