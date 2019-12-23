package nodeset

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testNodeSet struct {
	test   string
	result string
	length int
}

func TestNodeSetSimple(t *testing.T) {
	tests := []testNodeSet{
		testNodeSet{"cws-machin", "cws-machin", 1},
		testNodeSet{"cpn-d13-01", "cpn-d13-01", 1},
		testNodeSet{"cpn-d13-[01]", "cpn-d13-[01]", 1},
		testNodeSet{"cpn-d13-[01-10]", "cpn-d13-[01-10]", 10},
		testNodeSet{"cpn-k[08-09]-[02-24/2]-[01-02]", "cpn-k[08-09]-[02,04,06,08,10,12,14,16,18,20,22,24]-[01-02]", 48},
	}

	for _, nstest := range tests {
		n1, err := NewNodeSet(nstest.test)
		assert.Nil(t, err)
		assert.Equal(t, nstest.result, n1.String())
		assert.Equal(t, nstest.length, n1.Len())
	}
}

func TestNodeSetIterator(t *testing.T) {
	test := make([]string, 0)
	for i := 1; i < 11; i++ {
		test = append(test, fmt.Sprintf("cpn-d13-%02d", i))
	}

	n1, err := NewNodeSet("cpn-d13-[01-10]")
	assert.Nil(t, err)
	assert.Equal(t, 10, n1.Len())

	result := make([]string, 0)

	it := n1.Iterator()
	for it.Next() {
		result = append(result, it.Value())
	}

	assert.Equal(t, 10, len(result))
	assert.EqualValues(t, test, result)
}