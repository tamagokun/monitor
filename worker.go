package main

import (
	"fmt"
	"net/http"
	"os"
	"github.com/robfig/cron"
)

func main() {
	c := cron.New()
	c.AddFunc("0 5 * * * *",  func() { fmt.Println("Every 5 minutes") })
	c.AddFunc("@hourly",      func() { fmt.Println("Every hour") })
	c.AddFunc("@every 1h30m", func() { fmt.Println("Every hour thirty") })
	c.Start()
}
