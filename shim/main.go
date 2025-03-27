package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
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

	log := func(line string) {
		_, err = f.WriteString(fmt.Sprintf("%s: %s", now.Format(time.RFC3339), line))
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}

	log(fmt.Sprintf("args: %v\n", os.Args))

	// Get all the environment variables
	env := os.Environ()

	// write environment variables to the log file
	for _, e := range env {
		log(fmt.Sprintf("env: %s\n", e))
	}

	// write the current working directory to the log file

	cwd, err := os.Getwd()
	if err != nil {
		log(fmt.Sprintf("Error: %s\n", err))
		return
	}
	log(fmt.Sprintf("cwd: %s\n", cwd))

	// write the current user to the log file

	user, err := os.UserHomeDir()
	if err != nil {
		log(fmt.Sprintf("Error: %s\n", err))
		return
	}

	log(fmt.Sprintf("user: %s\n", user))
}