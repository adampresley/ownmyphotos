package services

func GetPointer[T comparable](val T) *T {
	var zeroValue T

	if val == zeroValue {
		return nil
	}

	return &val
}
