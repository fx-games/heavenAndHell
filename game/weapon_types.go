package game

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
)

func getBigAxeDamageLocations(state *common.State) func(attackerPos, targetPos geometry.Point, totalDamageForAllWeapons stats.Interval, weaponCount int) []common.LocationDamagePrediction {
	return func(attackerPos, targetPos geometry.Point, totalDamageForAllWeapons stats.Interval, weaponCount int) []common.LocationDamagePrediction {
		currentMap := state.GetMap()

		posIsValid := func(pos geometry.Point) bool {
			return currentMap.Contains(pos) && currentMap.IsActorAt(pos)
		}

		damageLocations := currentMap.GetFilteredNeighbors(attackerPos, posIsValid)
		if len(damageLocations) == 0 {
			damageLocations = append(damageLocations, targetPos)
		}

		var locationDamages []common.LocationDamagePrediction

		locationCount := len(damageLocations)
		damagePerLocation := totalDamageForAllWeapons.Div(locationCount)
		for _, damageLocation := range damageLocations {
			locationDamages = append(locationDamages, common.LocationDamagePrediction{Location: damageLocation, Damage: damagePerLocation})
		}

		return locationDamages
	}
}

func getSmallAxeDamageLocations(state *common.State) func(attackerPos, targetPos geometry.Point, totalDamageForAllWeapons stats.Interval, weaponCount int) []common.LocationDamagePrediction {
	return func(attackerPos, targetPos geometry.Point, totalDamageForAllWeapons stats.Interval, weaponCount int) []common.LocationDamagePrediction {
		damage := totalDamageForAllWeapons.Div(weaponCount)
		currentMap := state.GetMap()
		// we want to return the two locations which are right next to the targetPos left&right
		posIsValid := func(pos geometry.Point) bool {
			return currentMap.Contains(pos) && currentMap.IsActorAt(pos)
		}
		isDiagonal := attackerPos.X != targetPos.X && attackerPos.Y != targetPos.Y

		var damageLocations []geometry.Point
		// add the targetPos
		damageLocations = append(damageLocations, targetPos)

		if isDiagonal {
			left := geometry.Point{X: attackerPos.X, Y: targetPos.Y}
			right := geometry.Point{X: targetPos.X, Y: attackerPos.Y}
			if posIsValid(left) {
				damageLocations = append(damageLocations, left)
			}
			if posIsValid(right) {
				damageLocations = append(damageLocations, right)
			}
		} else {
			// check which axis is different
			if attackerPos.Y != targetPos.Y {
				// add the left and right of the targetPos
				left := geometry.Point{X: targetPos.X - 1, Y: targetPos.Y}
				right := geometry.Point{X: targetPos.X + 1, Y: targetPos.Y}
				if posIsValid(left) {
					damageLocations = append(damageLocations, left)
				}
				if posIsValid(right) {
					damageLocations = append(damageLocations, right)
				}
			} else if attackerPos.X != targetPos.X {
				// add the top and bottom of the targetPos
				top := geometry.Point{X: targetPos.X, Y: targetPos.Y - 1}
				bottom := geometry.Point{X: targetPos.X, Y: targetPos.Y + 1}
				if posIsValid(top) {
					damageLocations = append(damageLocations, top)
				}
				if posIsValid(bottom) {
					damageLocations = append(damageLocations, bottom)
				}
			}
		}

		var locationDamages []common.LocationDamagePrediction
		for _, damageLocation := range damageLocations {
			damagePerLocation := damage.Div(len(damageLocations))
			locationDamages = append(locationDamages, common.LocationDamagePrediction{Location: damageLocation, Damage: damagePerLocation})
		}

		return locationDamages
	}
}
