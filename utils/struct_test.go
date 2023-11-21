package utils

import (
	"fmt"
	"testing"
)

func TestAnyGet(t *testing.T) {
	type test struct {
		AA string
	}

	if AnyGet[string](test{AA: "123"}, "AA") != "123" {
		t.FailNow()
	}
	if AnyGet[string](&test{AA: "123"}, "AA") != "123" {
		t.FailNow()
	}

}

func TestAnySet(t *testing.T) {
	type aa struct {
		A string
	}

	a := aa{A: "123"}

	AnySet(&a, "321", "A")

	fmt.Println(a.A)
}
