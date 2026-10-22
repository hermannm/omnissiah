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
