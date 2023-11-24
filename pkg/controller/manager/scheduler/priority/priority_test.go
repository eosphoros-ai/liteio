package priority

import (
	"sort"
	"strconv"
	"testing"
)

func TestPriorityList(t *testing.T) {
	list := make(PriorityResultList, 10)
	for i := 0; i < len(list); i++ {
		list[i].Score = len(list) - i - 1
		list[i].NodeID = strconv.Itoa(i)
	}

	sort.Sort(sort.Reverse(list))

	t.Logf("%+v", list)
}
