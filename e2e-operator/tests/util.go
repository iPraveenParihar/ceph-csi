package tests

import (
	"fmt"
	"math/rand/v2"
)

func RandomName(prefix string) string { return fmt.Sprintf("%s-%06x", prefix, rand.Uint32()&0xffffff) }
