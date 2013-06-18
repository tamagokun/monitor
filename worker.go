package main

import (
	"fmt"
	"time"
	"io/ioutil"
	"encoding/json"
	"net/http"
)

type Config struct {
	Wait     int
	Timeout  int
	Sites    []string
}

func worker(id int, jobs <-chan string, results chan<- int) {
	for j := range jobs {
		fmt.Println("worker", id, "processing job", j)

		res, err := http.Get(string(j))
		if err != nil {
			fmt.Println("worker", id, "site", j, "is down!")
		}
		fmt.Println("worker", id, "site", j, "is up")
		results <- res.StatusCode
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func perform(config Config, jobs chan<- string, results <-chan int) {
	for j := 0; j < len(config.Sites); j++ {
		jobs <- config.Sites[j]
	}

	for a := 0; a < len(config.Sites); a++ {
		<- results
	}

	select {
		case <-time.After(time.Duration(config.Wait) * time.Second):
			perform(config, jobs, results)
	}
}

func main() {
	jobs := make(chan string, 100)
	results := make(chan int, 100)
	done := make(chan bool, 1)

	f, err := ioutil.ReadFile("./config.json")
	check(err)
	var config Config

	json_err := json.Unmarshal(f, &config)
	check(json_err)

	for w := 1; w <= 3; w++ {
		go worker(w, jobs, results)
	}

	perform(config, jobs, results)
	<- done
}
