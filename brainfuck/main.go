package main

import (
	"brainfuck/bf"
	"flag"
	"fmt"
	"os"
)

var filename string

func init() {
	flag.StringVar(&filename, "file", "", "brainfuck source file")
}

func main() {
	flag.Parse()
	if filename == "" {
		fmt.Println("Please provide a filename using the -file flag.")
		return
	}

	input, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	bf.Run(string(input), os.Stdin, os.Stdout)
}
