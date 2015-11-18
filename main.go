package main

import (
	"crypto/tls"
	"fmt"
	"github.com/aryann/difflib"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	currentContent      map[string][]string
	currentContentMutex sync.Mutex
	pages               []string
	threshold           uint64
	interval            uint64
	smtp_user           string
	smtp_password       string
	smtp_host           string
	smtp_from           string
	smtp_to             string
)

func main() {
	currentContent = make(map[string][]string)
	pages = make([]string, 0)
	threshold = 1
	interval = 60

	parseEnv()

	go scanPages()
	for range time.Tick(time.Duration(interval) * time.Second) {
		go scanPages()
	}

}

func scanPages() {
	for _, page := range pages {
		go scanPage(page)
	}
	return
}

func scanPage(page string) {
	log.Printf("Begin scan of %s.\n", page)

	res, err := http.Get(page)
	if err != nil {
		log.Printf("%s: %v", page, err)
	}
	if res.StatusCode != 200 {
		log.Printf("%s: Got status code %d.", page, res.StatusCode)
	}
	response, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Printf("%s: %v", page, err)
	}
	responseLines := toLines(string(response))
	previousVersion, ok := currentContent[page]
	var comparison string
	if !ok {
		log.Printf("First scan of %s was successful, no previous data for comparison available.\n", page)
	} else {
		comparison = compare(previousVersion, responseLines)
	}
	currentContentMutex.Lock()
	currentContent[page] = responseLines
	currentContentMutex.Unlock()

	if comparison != "" && uint64(len(toLines(comparison))) <= (threshold*2)+1 {
		log.Printf("Change of %s detected, but does not exceed threshold of %d lines.", page, threshold)
	} else if comparison != "" {
		log.Printf("Change of %s detected:\n\n---------------%s\n---------------\n\n", page, comparison)
		err := sendNotification(page, comparison)
		if err != nil {
			log.Printf("%s: Got error while sending notification %v.", page, err)
		}
	} else {
		log.Printf("No change of %s detected.\n", page)
	}
	return
}

func compare(oldVersion []string, newVersion []string) string {
	diff := difflib.Diff(oldVersion, newVersion)
	output := ""
	for _, v := range diff {
		if v.Delta == difflib.Common {
			continue
		}
		output += "\n" + v.String()
	}
	return output
}

func sendNotification(page string, comparison string) error {
	from := mail.Address{"", smtp_from}
	to := mail.Address{"", smtp_to}
	subj := fmt.Sprintf("Change detected: %s", page)
	body := fmt.Sprintf("Change on page %s detected:\n\n%s", page, comparison)

	// Setup headers
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = subj

	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Connect to the SMTP Server
	host, _, _ := net.SplitHostPort(smtp_host)

	auth := smtp.PlainAuth("", smtp_user, smtp_password, host)

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	conn, err := tls.Dial("tcp", smtp_host, tlsconfig)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		return err
	}

	// To && From
	if err = c.Mail(from.Address); err != nil {
		return err
	}

	if err = c.Rcpt(to.Address); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	c.Quit()

	return nil

}

func parseEnv() {
	// Pages
	env_pages := os.Getenv("PAGES")
	if env_pages == "" {
		log.Fatal("Environment variable PAGES is empty.")
	}
	pages = append(pages, strings.Split(env_pages, ",")...)

	// Interval
	env_interval := os.Getenv("INTERVAL")
	if env_interval == "" {
		log.Fatal("Environment variable INTERVAL is empty.")
	}
	interval_parsed, err := strconv.ParseUint(env_interval, 10, 64)
	if err != nil {
		log.Fatal("Environment variable INTERVAL is no valid integer.")
	}
	interval = interval_parsed

	// Threshold
	env_threshold := os.Getenv("THRESHOLD")
	if env_threshold != "" {
		interval_threshold, err := strconv.ParseUint(env_threshold, 10, 64)
		if err != nil {
			log.Fatal("Environment variable THRESHOLD is no valid integer.")
		}
		threshold = interval_threshold
	}

	// Mail
	env_smtp_user := os.Getenv("SMTP_USER")
	if env_smtp_user == "" {
		log.Fatal("Environment variable SMTP_USER is empty.")
	}
	smtp_user = env_smtp_user

	env_smtp_password := os.Getenv("SMTP_PASSWORD")
	if env_smtp_password == "" {
		log.Fatal("Environment variable SMTP_PASSWORD is empty.")
	}
	smtp_password = env_smtp_password

	env_smtp_host := os.Getenv("SMTP_HOST")
	if env_smtp_host == "" {
		log.Fatal("Environment variable SMTP_HOST is empty.")
	}
	smtp_host = env_smtp_host

	env_smtp_from := os.Getenv("SMTP_FROM")
	if env_smtp_from == "" {
		log.Fatal("Environment variable SMTP_FROM is empty.")
	}
	smtp_from = env_smtp_from

	env_smtp_to := os.Getenv("SMTP_TO")
	if env_smtp_to == "" {
		log.Fatal("Environment variable SMTP_TO is empty.")
	}
	smtp_to = env_smtp_to
}

func toLines(text string) []string {
	return strings.Split(text, "\n")
}
