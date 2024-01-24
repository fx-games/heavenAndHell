package common

import "rpg/geometry"

type Camera interface {
	CenterOnFloat(pos geometry.PointF)
}
