package main

import (
	"encoding/csv"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Owner         string `yaml:"owner"`
	Repo          string `yaml:"repo"`
	StatusWebsite struct {
		CName   string `yaml:"cname"`
		LogoURL string `yaml:"logoUrl"`
		Name    string `yaml:"name"`
	} `yaml:"status-website"`
	Groups []Group `yaml:"groups"`
}

type Group struct {
	Name  string `yaml:"name"`
	Slug  string `yaml:"slug"`
	Sites []Site `yaml:"sites"`
}

type Site struct {
	Name string `yaml:"name"`
	Desc string `yaml:"desc"`
	Slug string `yaml:"slug"`
}

type StatusEntry struct {
	Time   time.Time `csv:"time"`
	Status string    `csv:"status"`
}

const (
	maxDays = 45
)

func main() {
	config := readConfig("config.yml")
	now := time.Now().UTC()
	for _, group := range config.Groups {
		groupStatus := "success"
		for _, site := range group.Sites {
			siteStatus := checkSite(site, now)
			if siteStatus == "error" {
				groupStatus = "error"
			}
		}
		checkGroup(group, now, groupStatus)
	}
}

func readConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}
	return config
}

func checkSite(site Site, now time.Time) string {
	url := site.Desc
	resp, err := http.Get(url)
	status := "error"
	if err == nil && resp.StatusCode == http.StatusOK {
		status = "success"
	}
	if resp != nil {
		resp.Body.Close()
	}

	logFilePath := filepath.Join("logs", site.Slug+".csv")
	ensureLogFile(logFilePath)
	writeStatus(logFilePath, now, status)
	return status
}

func checkGroup(group Group, now time.Time, status string) {
	logFilePath := filepath.Join("logs", group.Slug+".csv")
	ensureLogFile(logFilePath)
	writeStatus(logFilePath, now, status)
}

func ensureLogFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		writer := csv.NewWriter(file)
		writer.Write([]string{"time", "status"})
		writer.Flush()
	}
}

func writeStatus(path string, now time.Time, status string) {
	entries := readLogEntries(path)
	entries = removeOldDates(entries)

	if (now.Hour() == 0 && now.Minute() < 10) || (now.Hour() == 23 && now.Minute() >= 50) { //if this is the first or last run of the day
		entries = append(entries, StatusEntry{now, status})
	} else if len(entries) == 0 { //if there is no record before
		entries = append(entries, StatusEntry{now, status})
	} else if entries[len(entries)-1].Status != status { //if the previous record is different from
		entries = append(entries, StatusEntry{now, status})
	}

	writeLogEntries(path, entries)
}

func readLogEntries(path string) []StatusEntry {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var entries []StatusEntry
	reader := csv.NewReader(file)
	reader.Read() // skip header
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		time, err := time.Parse(time.RFC3339, record[0])
		if err != nil {
			panic(err)
		}
		entries = append(entries, StatusEntry{Time: time, Status: record[1]})
	}
	return entries
}

func removeOldDates(entries []StatusEntry) []StatusEntry {
	now := time.Now().UTC()
	dateLimit := now.AddDate(0, 0, -maxDays)
	
	var filteredEntries []StatusEntry
	for _, entry := range entries {
		if entry.Time.After(dateLimit) || entry.Time.Equal(dateLimit) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	return filteredEntries
}

func writeLogEntries(path string, entries []StatusEntry) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"time", "status"})
	for _, entry := range entries {
		writer.Write([]string{entry.Time.Format(time.RFC3339), entry.Status})
	}
	writer.Flush()
}
