package thread

func NewResultResponse[T any]() ResultResponse[T] {
	return make(ResultResponse[T])
}

type ResultResponse[T any] chan Result[T]

func (c ResultResponse[T]) Receive() (T, error) {
	r := <-c.Ch()
	return r.Value, r.Err
}

func (c ResultResponse[T]) Ch() chan Result[T] {
	return (chan Result[T])(c)
}

func (c ResultResponse[T]) Ok(v T) {
	c.Ch() <- Result[T]{Value: v}
	close(c.Ch())
}

func (c ResultResponse[T]) Err(err error) {
	c.Ch() <- Result[T]{Err: err}
	close(c.Ch())
}

type Result[T any] struct {
	Value T
	Err   error
}
