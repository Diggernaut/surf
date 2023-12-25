package jar

import (
	"sync"
	"time"

	"github.com/diggernaut/goquery"
)

type AsyncDom struct {
	T time.Time
	D *goquery.Document
}
type AsyncStore struct {
	sync.Mutex
	doms map[string]*AsyncDom
}

func NewAsyncStore() *AsyncStore {
	return &AsyncStore{doms: make(map[string]*AsyncDom)}
}
func (a *AsyncStore) Set(name string, val *goquery.Document) {
	a.Lock()
	defer a.Unlock()
	a.doms[name] = &AsyncDom{T: time.Now(), D: val}
}
func (a *AsyncStore) Get(name string) *AsyncDom {
	a.Lock()
	defer a.Unlock()
	if v, ok := a.doms[name]; ok {
		return v
	}
	return &AsyncDom{}
}
