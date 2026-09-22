package common

import "rpg/src/geometry"

type Camera interface {
	CenterOnFloat(pos geometry.PointF)
}
