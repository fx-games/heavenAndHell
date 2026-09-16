package common

type Visibility struct {
	visibleActors map[*Actor]bool
}

func (v *Visibility) IsVisible(actor *Actor) bool {
	_, exists := v.visibleActors[actor]
	return exists && actor.IsVisible()
}

func (v *Visibility) GetVisibleActors() map[*Actor]bool {
	seen := make(map[*Actor]bool, len(v.visibleActors))
	for actor := range v.visibleActors {
		if actor.IsVisible() {
			seen[actor] = true
		}
	}
	return seen
}

func (v *Visibility) Set(visible map[*Actor]bool) {
	v.visibleActors = visible
}

func (v *Visibility) GetVisibleEnemyCount(isEnemy func(*Actor) bool) int {
	count := 0
	for actor, _ := range v.visibleActors {
		if actor.IsVisible() && isEnemy(actor) {
			count++
		}
	}
	return count
}

func (v *Visibility) RemoveActor(victim *Actor) {
	delete(v.visibleActors, victim)
}

func (v *Visibility) GetVisibleEnemies() []*Actor {
	enemies := make([]*Actor, 0)
	for actor, _ := range v.visibleActors {
		if actor.IsVisible() && actor.IsHostile() {
			enemies = append(enemies, actor)
		}
	}
	return enemies

}

func NewVisibility() *Visibility {
	return &Visibility{
		visibleActors: make(map[*Actor]bool),
	}
}
