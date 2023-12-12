package filter

import (
	"fmt"
	"sync"
)

var (
	regLock sync.Mutex

	filtersMap = make(map[string]PredicateFunc, 0)
)

func init() {
	RegisterFilter("Basic", BasicFilterFunc)
	RegisterFilter("Affinity", AffinityFilterFunc)
	RegisterFilter("MinLocalStorage", MinLocalStorageFilterFunc)
}

func RegisterFilter(name string, filter PredicateFunc) {
	regLock.Lock()
	defer regLock.Unlock()

	filtersMap[name] = filter
}

func GetFilterByName(name string) (filter PredicateFunc, err error) {
	regLock.Lock()
	defer regLock.Unlock()

	if f, has := filtersMap[name]; has {
		return f, nil
	}
	return nil, fmt.Errorf("not found filter by name %s", name)
}
