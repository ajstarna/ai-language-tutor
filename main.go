package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

func main() {

	config := loadConfig()
	fmt.Println(config)

	tutor, err := NewTutor(config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "quit" {
			break
		}
		modelOutput, err := tutor.callModel(line)
		if err != nil {
			fmt.Println("error:", err)
			fmt.Print("> ")
			continue
		}
		fmt.Println(modelOutput)
		fmt.Print("> ")
	}

}
