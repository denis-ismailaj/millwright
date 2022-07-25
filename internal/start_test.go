package internal

import (
	"testing"
)

func TestCheckConfiguration(t *testing.T) {
	a := &Component{
		dependencies: []*Component{},
	}

	b := &Component{
		dependencies: []*Component{a},
	}

	c := &Component{
		dependencies: []*Component{b},
	}

	components := []*Component{a, b, c}

	err := checkConfiguration(components)
	if err != nil {
		t.Fatal("Check failed but should have passed.")
	}

	b.dependencies = append(b.dependencies, c)

	err = checkConfiguration(components)
	if err == nil {
		t.Fatal("Check should have failed.")
	}
}
