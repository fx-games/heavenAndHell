package common

type BaseTimedBeing struct {
	energy    int
	onTurn    func()
	baseSpeed int
}

func NewBaseTimedBeing() *BaseTimedBeing {
	return &BaseTimedBeing{
		energy:    0,
		baseSpeed: 10,
	}
}
func (d *BaseTimedBeing) SetOnTurn(turn func()) {
	d.onTurn = turn
}
func (d *BaseTimedBeing) IsAlive() bool {
	return true
}
func (d *BaseTimedBeing) GetEnergy() int {
	return d.energy
}

func (d *BaseTimedBeing) AddEnergy(energy int) {
	d.energy += energy
}

func (d *BaseTimedBeing) RemoveEnergy(used int) {
	d.energy -= used
}

func (d *BaseTimedBeing) GetBaseSpeed() int {
	return d.baseSpeed
}

func (d *BaseTimedBeing) Turn() {
	if d.onTurn != nil {
		d.onTurn()
	}
}
