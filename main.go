package main

import (
	"fmt"
)

func main() {
	prueba := make([]int, 4, 8)
	prueba = append(prueba, 1, 2, 3, 4)
	subslice := prueba[len(prueba):cap(prueba)]
	fmt.Println(subslice)
	subslice = append(subslice, 1, 2)
	fmt.Println(subslice)
}
