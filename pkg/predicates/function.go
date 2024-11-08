package predicates

// Predicate is a predicate function for a type.
type Predicate[T any] func(T) bool

// AllOf returns a predicate that is true if all of the given predicates are true.
func AllOf[T any](predicates ...Predicate[T]) Predicate[T] {
	return func(t T) bool {
		for _, predicate := range predicates {
			if !predicate(t) {
				return false
			}
		}
		return true
	}
}

// Filter returns a slice of elements that match the predicate.
func Filter[T any](slice []T, predicate Predicate[T]) []T {
	if predicate == nil {
		return slice
	}

	var result []T
	for _, element := range slice {
		if predicate(element) {
			result = append(result, element)
		}
	}
	return result
}
