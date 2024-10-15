package test

import "iter"

func Take[T any](n int, iterator iter.Seq2[T, error]) ([]T, error) {
	var result []T

	count := 0
	for item, err := range iterator {
		if err != nil {
			return nil, err
		}

		result = append(result, item)
		count++

		if count >= n {
			break
		}
	}

	return result, nil
}
