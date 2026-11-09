package common

type ctxKey int

const (
	KeySession ctxKey = iota
	KeyInstance
	KeySpace
	KeyController
	KeyIslandStore
	KeyIslandWidget
)

type IslandWidget struct {
	Name string
	ID   uint
}
