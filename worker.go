package main

import (
	"fmt"
	"time"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"
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

type Site struct {
	Url       string
	Status    int
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var sites = make([]Site, 0)
var config Config

/*
 * sends email when site status changes
 */
func notify(status string, site Site) {
	api_key := os.Getenv("POSTMARK_API_KEY")
	mail_server := os.Getenv("POSTMARK_SMTP_SERVER") + ":25"

	auth := smtp.CRAMMD5Auth(
		api_key,
		api_key,
	)

	from := "monitor@ripeworks.com"
	stat := http.StatusText(site.Status)
	time := time.Now().Format("Mon Jan 02 15:04:05 2006")
	body := "<h1>Status for " + site.Url + "</h1>"
	body += "<p>" + status + ": " + stat + " @ " + time + "</p>"

	header := make(map[string]string)
	header["From"] = from
	header["To"] = config.Recipient
	header["Subject"] = "Monitor - " + site.Url + " - " + status
	header["Content-Type"] = "text/html; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	err := smtp.SendMail(
		mail_server,
		auth,
		from,
		[]string{config.Recipient},
		[]byte(message),
	)
	if err != nil {
		fmt.Println("Unable to notify via email")
		fmt.Println(err)
	}
}

/*
 * recursive func that sends websites to the jobs queue.
 * runs every config.Wait seconds.
 */
func perform(jobs chan<- Site, results <-chan int) {
	for j := 0; j < len(sites); j++ {
		jobs <- sites[j]
	}

	for a := 0; a < len(config.Sites); a++ {
		<- results
	}

	select {
		case <-time.After(time.Duration(config.Wait) * time.Second):
			perform(jobs, results)
	}
}

/*
 * goroutine that processes a website and reports its status.
 */
func worker(id int, jobs <-chan Site, results chan<- int) {
	for j := range jobs {
		res, err := http.Get(j.Url)
		status := 503 // Something went wrong, use 'Gateway Timeout'
		if res != nil {
			res.Body.Close()
			status = res.StatusCode
		}
		if err != nil || status > 399 {
			fmt.Println("[", id, "] -", j.Url, "is DOWN")
			j.Status = status
			notify("DOWN", j)
		}else {
			if status < 400 && j.Status > 399 {
				notify("BACK UP", j)
			}
			if status != j.Status {
				fmt.Println("[", id, "] -", j.Url, "is UP -", status)
			}
			j.Status = status
		}
		results <- j.Status
	}
}

func main() {
	jobs := make(chan Site)
	results := make(chan int)
	done := make(chan bool, 1)

	f, err := ioutil.ReadFile("./config.json")
	check(err)

	json_err := json.Unmarshal(f, &config)
	check(json_err)

	for w := 1; w <= 3; w++ {
		go worker(w, jobs, results)
	}

	for s := 0; s < len(config.Sites); s++ {
		sites = append(sites, Site{Url: config.Sites[s], Status: 0})
	}

	perform(jobs, results)
	<- done
}
