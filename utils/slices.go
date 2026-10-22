package utils

func Find[E any](slice []E, match func(E) bool) (found E, ok bool) {
	for _, e := range slice {
		if match(e) {
			return e, true
		}
	}
	var e E
	return e, false
}

func Map[E any, E2 any](slice []E, mapper func(E) E2) []E2 {
	newSlice := make([]E2, 0, len(slice))
	for _, e := range slice {
		newSlice = append(newSlice, mapper(e))
	}
	return newSlice
}
