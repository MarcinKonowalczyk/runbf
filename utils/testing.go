package utils

import (
	"runtime"
	"testing"
)

func getParentInfo() (string, int) {
	parent, _, _, _ := runtime.Caller(2)
	info := runtime.FuncForPC(parent)
	file, line := info.FileLine(parent)
	return file, line
}

// Test helper
func Assert(t *testing.T, predicate bool, msg string) {
	if !predicate {
		file, line := getParentInfo()
		t.Errorf(msg+" in %s:%d", file, line)
	}
}

func AssertEqual[T comparable](t *testing.T, a T, b T) {
	if a != b {
		file, line := getParentInfo()
		t.Errorf("Expected %v == %v (%T) in %s:%d", a, b, a, file, line)
	}
}

func AssertNotEqual[T comparable](t *testing.T, a T, b T) {
	if a == b {
		file, line := getParentInfo()
		t.Errorf("Expected %v != %v (%T) in %s:%d", a, b, a, file, line)
	}
}

// Assert that error is nil
func AssertNoError(t *testing.T, err error) {
	if err != nil {
		file, line := getParentInfo()
		t.Errorf("Expected no error, got '%v' in %s:%d", err, file, line)
	}
}

// Assert that an error is not nil
func AssertError(t *testing.T, err error) {
	if err == nil {
		file, line := getParentInfo()
		t.Errorf("Expected error, got '%v' in %s:%d", err, file, line)
	}
}

// // Compare two values using a custom comparator function.
func AssertEqualWithComparator[T any](t *testing.T, a T, b T, comparator func(T, T) bool) {
	if !comparator(a, b) {
		file, line := getParentInfo()
		t.Errorf("Expected %v == %v (%T) in %s:%d", a, b, a, file, line)
	}
}

func CompareArrays[T comparable](a []T, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func CompareMaps[T comparable, V comparable](a map[T]V, b map[T]V) bool {
	if len(a) != len(b) {
		return false
	}
	var vb V
	var ok bool

	// NOTE: the range on a map is in random order
	for k, va := range a {
		// Check if key exists in b
		if vb, ok = b[k]; !ok {
			return false
		}
		// Check if value is the same
		if va != vb {
			return false
		}
	}

	// All keys of a exist in b, and a and b have the same length, hence they
	// must have the same keys
	return true
}

// Utility functions for comparing arrays. Equivalent to AssertEqualWithComparator
// where the comparator is CompareArrays.
func AssertEqualArrays[T comparable](t *testing.T, a []T, b []T) {
	AssertEqualWithComparator(t, a, b, CompareArrays)
}

// Utility functions for comparing maps. Equivalent to AssertEqualWithComparator
// where the comparator is CompareMaps.
func AssertEqualMaps[T comparable, V comparable](t *testing.T, a map[T]V, b map[T]V) {
	AssertEqualWithComparator(t, a, b, CompareMaps)
}

// Check if two arrays are equal, regardless of the order of the elements.
func CompareArraysUnordered[T comparable](a []T, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	am := make(map[T]int) // map from element to count
	for _, e := range a {
		am[e]++
	}
	// Iterate over b, decrementing the count of each element in am.
	for _, e := range b {
		if am[e] == 0 {
			return false
		}
		am[e]--
	}
	return true
}

func AssertEqualArraysUnordered[T comparable](t *testing.T, a []T, b []T) {
	AssertEqualWithComparator(t, a, b, CompareArraysUnordered)
}
