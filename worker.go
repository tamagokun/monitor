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
  From      string
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
func notify(status string, site *Site) {
  api_key := os.Getenv("POSTMARK_API_KEY")
  mail_server := os.Getenv("POSTMARK_SMTP_SERVER") + ":25"

  auth := smtp.CRAMMD5Auth(
    api_key,
    api_key,
  )

  from := config.From
  stat := http.StatusText(site.Status)
  time := time.Now().Format("Mon Jan 02 15:04:05 2006")
  body := "<center><br>"
  body += "<h4>" + site.Url + " is " + status + "</h4>"
  body += fmt.Sprintf("<p><strong>%s (%d)</strong> @ %s</p>", stat, site.Status, time)
  body += "<br></center>"

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
func perform(jobs chan<- *Site, results <-chan int) {
  for j := 0; j < len(sites); j++ {
    jobs <- &sites[j]
  }

  for a := 0; a < len(sites); a++ {
    <-results
  }

  select {
    case <-time.After(time.Duration(config.Wait) * time.Second):
      perform(jobs, results)
  }
}

/*
 * goroutine that processes a website and reports its status.
 */
func worker(id int, jobs <-chan *Site, results chan<- int) {

  for j := range jobs {
    status := check_http_status(j.Url, true)
    if status != j.Status {
      // status changed, lets notify
      state := "UP"
      if status <= 399 && j.Status > 399 { state = "BACK UP" }
      if status > 399 { state = "DOWN" }
      fmt.Println("[", id, "] - ", j.Url, "is ", state, " -", status)
      j.Status = status
      if state == "BACK UP" || state == "DOWN" {
        notify(state, j)
      }
    }
    j.Status = status
    results <- j.Status
  }
}

/*
 * send GET request to url.
 * Returns status code.
 */
func check_http_status(url string, retry bool) (int) {
  status := 408 // Something went wrong, default to 'ClientRequestTimeout'
  timeout := time.Duration(config.Timeout) * time.Second
  client := http.Client{Timeout: timeout}

  res, err := client.Head(url)
  if err != nil {
    fmt.Println("Unable to check")
    fmt.Println(err)
    if retry { return check_http_status(url, false) }
  }
  if res != nil {
    defer res.Body.Close()
    status = res.StatusCode
  }
  return status
}

func main() {
  jobs := make(chan *Site, 100)
  results := make(chan int, 100)
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
