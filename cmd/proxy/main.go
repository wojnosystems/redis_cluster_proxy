package main

import (
	"log"
	"os"
)

func main() {
	err := buildArguments().Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
