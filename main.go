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
		modelOutput, _ := tutor.callModel(line)
		fmt.Println(modelOutput)
		fmt.Print("> ")
	}

}
