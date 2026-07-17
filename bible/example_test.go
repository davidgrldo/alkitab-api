package bible_test

import (
	"fmt"

	"github.com/davidgrldo/alkitab-api/bible"
	"github.com/davidgrldo/alkitab-api/local"
)

// The embedded public-domain sample works with zero configuration; point
// local.New at a data directory to serve full translations (BYOD).
func Example() {
	src, err := local.New("")
	if err != nil {
		panic(err)
	}
	eng := bible.New(src)

	ch, err := eng.Chapter("kjv", "3john", 1)
	if err != nil {
		panic(err)
	}
	for _, v := range ch.Verses {
		if v.Number == 4 {
			fmt.Println(v.Content)
		}
	}
	// Output: I have no greater joy than to hear that my children walk in truth.
}
