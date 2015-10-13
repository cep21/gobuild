package _example

import (
	"fmt"
	"os"
)

func Undocumented() {
}

func ineff() int {
	i := 3
	j := 3
	i = 5 * j
	j = 4
	return i
}

type aligncheckStruct struct {
	small1 bool
	big    int64
	small2 bool
}

func errCheck() {
	os.Stderr.WriteString("errCheck")
}

func vet() {
	fmt.Printf("Hello %s")
}

func vetShadow(ar []int) int {
	i := 3
	for j := range ar {
		i := j
		if i > 2 {
			fmt.Printf("Shadow?\n")
		}
	}
	return i
}

func gocycloDupl() {
	i := 3
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
}


func gocycloDupl2() {
	i := 3
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
	if i > 0 && i > 1 && i > 2 && i > 3 {
		fmt.Println("A")
	}
}
