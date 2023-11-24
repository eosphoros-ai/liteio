package misc

import "sync"

// Set datastructure for strings
type Set interface {
	Contains(elem string) bool
	Add(elem string)
	Union(another Set) Set
	Intersect(another Set) Set
	// Difference returns items in s but not in another
	Difference(another Set) Set
	IsSupersetOf(another Set) bool
	Equal(another Set) bool
	Size() int
	Remove(elem string) bool
	Values() []string
}

// Map based Set implementation
type MapSet struct {
	_data map[string]struct{}
	lock  sync.RWMutex
}

// NewEmptySet of strings
func NewEmptySet() Set {
	return &MapSet{
		_data: make(map[string]struct{}),
	}
}

// Creates a new Set from a slice of strings
func FromSlice(slice []string) Set {
	set := NewEmptySet()
	for _, elem := range slice {
		set.Add(elem)
	}

	return set
}

// Checks for an existence of an element
func (s *MapSet) Contains(elem string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	_, present := s._data[elem]
	return present
}

// Add an element to the Set
func (s *MapSet) Add(elem string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s._data[elem] = struct{}{}
}

// Union another Set to this set and returns that
func (s *MapSet) Union(another Set) Set {
	union := FromSlice(s.Values())
	for _, value := range another.Values() {
		s.lock.Lock()
		union.Add(value)
		s.lock.Unlock()
	}
	return union
}

// Intersect another Set to this Set and returns that
func (s *MapSet) Intersect(another Set) Set {
	intersection := NewEmptySet()
	for _, elem := range another.Values() {
		if s.Contains(elem) {
			s.lock.Lock()
			intersection.Add(elem)
			s.lock.Unlock()
		}
	}
	return intersection
}

// Difference returns items in s but not in another
func (s *MapSet) Difference(another Set) Set {
	//	intersect := s.Intersect(another)
	ret := NewEmptySet()
	for _, elem := range s.Values() {
		if !another.Contains(elem) {
			s.lock.Lock()
			ret.Add(elem)
			s.lock.Unlock()
		}
	}
	return ret
}

// IsSupersetOf another Set
func (s *MapSet) IsSupersetOf(another Set) bool {
	found := true
	for _, elem := range another.Values() {
		found = found && s.Contains(elem)
	}
	return found
}

// Equal another Set
func (s *MapSet) Equal(another Set) bool {
	found := s.Size() == another.Size()
	if found {
		for _, elem := range another.Values() {
			found = found && s.Contains(elem)
		}
	}
	return found
}

// Remove an element if it exists in the Set
// Returns if the value was present and removed
func (s *MapSet) Remove(elem string) bool {
	found := s.Contains(elem)
	s.lock.Lock()
	delete(s._data, elem)
	s.lock.Unlock()
	return found
}

// Values of the underlying set
func (s *MapSet) Values() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var values []string
	for key := range s._data {
		values = append(values, key)
	}

	return values
}

// Size of the set
func (s *MapSet) Size() int {
	return len(s._data)
}
