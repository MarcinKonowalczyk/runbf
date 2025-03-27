package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// get all commandline arguments
	fmt.Println("os.Args:", os.Args)
	// figure out where the shim is located
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	log_file := filepath.Join(filepath.Dir(path), "shim.log")
	// write arguments to the log file
	f, err := os.OpenFile(log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer f.Close()

	now := time.Now()
	line := fmt.Sprintf("os.Args: %v\n", os.Args)

	_, err = f.WriteString(fmt.Sprintf("%s: %s", now.Format(time.RFC3339), line))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}