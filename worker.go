package main

import (
	"fmt"
	"time"
	"io/ioutil"
	"encoding/json"
	"net/http"
	"net/smtp"
	"os"
)

type Config struct {
	Wait      int
	Timeout   int
	Recipient string
	Sites     []string
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func notify(to string, status int, site string) {
	api_key := os.Getenv("POSTMARK_API_KEY")
	mail_server := os.Getenv("POSTMARK_SMTP_SERVER") + ":25"

	auth := smtp.PlainAuth(
		"",
		api_key,
		api_key,
		"stark-chamber-8136.herokuapp.com",
	)

	time := time.Now().Format("Mon Jan _02 15:04:05 2006")
	msg := http.StatusText(status)
	if status == -1 {
		msg = "Request Timeout"
	}

	err := smtp.SendMail(
		mail_server,
		auth,
		"monitor@ripeworks.com",
		[]string{to},
		[]byte(site + " reported as -- " + msg + " @ " + time),
	)
	if err != nil {
		panic(err)
	}
}

/*
 * recursive func that sends websites to the jobs queue.
 * runs every config.Wait seconds.
 */
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

/*
 * goroutine that processes a website and reports its status./
 */
func worker(id int, jobs <-chan string, results chan<- int) {
	for j := range jobs {
		fmt.Println("worker", id, "processing job", j)

		res, err := http.Get(string(j))
		if err != nil {
			fmt.Println("worker", id, "site", j, "is down!")
			notify("mike@ripeworks.com", 200, j)
		}else {
			fmt.Println("worker", id, "site", j, "is up")
		}
		results <- res.StatusCode
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
