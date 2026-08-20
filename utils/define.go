package utils

type Dot [2]int

func NewDot(x int, y int) Dot {
	return Dot{x, y}
}

func (d Dot) X() int {
	return d[0]
}

func (d Dot) Y() int {
	return d[1]
}
