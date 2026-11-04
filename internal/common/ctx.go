package common

type ctxKey int

const (
	KeySession ctxKey = iota
	KeyInstance
	KeySpace
	KeyMessageStorage
	KeyFileStorage
	KeyController
	KeyIslandStore
)
