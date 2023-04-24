package set

type Set[V comparable] map[V]struct{}

func New[V comparable](values ...V) Set[V] {
	s := Set[V]{}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (s Set[V]) Add(values ...V) {
	for _, v := range values {
		s[v] = struct{}{}
	}
}

func (s Set[V]) Has(v V) bool {
	_, ok := s[v]
	return ok
}

func (s Set[V]) Union(other Set[V]) Set[V] {
	newSet := Set[V]{}
	for key := range other {
		if other.Has(key) {
			newSet.Add(key)
		}
	}
	return newSet
}

func (s Set[V]) Remove(v V) {
	delete(s, v)
}
