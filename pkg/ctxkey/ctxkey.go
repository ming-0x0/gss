package ctxkey

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return k.name
}

var (
	TransactionContextKey = &contextKey{name: "transaction"}
)
