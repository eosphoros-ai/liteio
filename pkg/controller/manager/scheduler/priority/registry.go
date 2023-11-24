package priority

import (
	"fmt"
	"sync"
)

var (
	regLock sync.Mutex

	prioritiesMap = make(map[string]PriorityFunc, 0)
)

func init() {
	RegisterPriorityFunc("LeastResource", PriorityByLeastResource)
	RegisterPriorityFunc("PositionAdvice", PriorityByPositionAdivce)
}

func RegisterPriorityFunc(name string, filter PriorityFunc) {
	regLock.Lock()
	defer regLock.Unlock()

	prioritiesMap[name] = filter
}

func GetPriorityByName(name string) (filter PriorityFunc, err error) {
	regLock.Lock()
	defer regLock.Unlock()

	if f, has := prioritiesMap[name]; has {
		return f, nil
	}
	return nil, fmt.Errorf("not found priority by name %s", name)
}
