package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const ENDPOINT = ""
const DIRECTORY = "./content/content"
const USE_FILENAME_AS_URL = false

type PostRequest struct {
	filename string
	date     string
	title    string
	content  string
	status   string
	url      string
}

func formatJSON(data []byte) (string, error) {
	var out bytes.Buffer

	err := json.Indent(&out, data, "", " ")
	if err != nil {
		return "", err
	}

	d := out.Bytes()
	return string(d), nil
}

func generateUrl(time time.Time, title string, filename string) string {
	if USE_FILENAME_AS_URL {
		splitFilename := strings.Split(filename, ".")
		splitFilename = splitFilename[:len(splitFilename)-1]
		return strings.Join(splitFilename, ".")
	}

	whitespaceRegex := regexp.MustCompile(`[^\w\s-]`)
	hyphenRegex := regexp.MustCompile(`[\s_-]+`)
	duplicateRegex := regexp.MustCompile(`^-+|-+$`)

	slug := strings.ToLower(title)
	slug = strings.TrimSpace(slug)
	slug = strings.ReplaceAll(slug, "ü", "ue")
	slug = strings.ReplaceAll(slug, "ä", "ae")
	slug = strings.ReplaceAll(slug, "ö", "öe")
	slug = strings.ReplaceAll(slug, "ß", "ss")
	slug = whitespaceRegex.ReplaceAllString(slug, "")
	slug = hyphenRegex.ReplaceAllString(slug, "-")
	slug = duplicateRegex.ReplaceAllString(slug, "")

	if len(slug) < 40 {
		return time.Format("2006-01-02") + "-" + slug
	}

	splitSlug := strings.Split(slug[:40], "-")
	splitSlug = splitSlug[:len(splitSlug)-1]

	return time.Format("2006-01-02") + "-" + strings.Join(splitSlug, "-")
}

func validateItem(item fs.DirEntry) (PostRequest, error) {
	postRequest := PostRequest{
		filename: item.Name(),
	}

	dateRegex := regexp.MustCompile(`date *= *"?([^"\n]+)"?`)
	titleRegex := regexp.MustCompile(`title *= *"([^"]+)"`)
	draftRegex := regexp.MustCompile(`draft *= *([^\n]+)`)

	file, err := os.ReadFile("./content/content/" + item.Name())
	if err != nil {
		return postRequest, err
	}

	// Clean the file content
	fileContent := string(file)
	fileContent = strings.TrimSpace(fileContent)
	fileContent = strings.Trim(fileContent, "+")
	fileContent = strings.TrimSpace(fileContent)

	// Split into header and body
	split := strings.Split(fileContent, "+++")
	header, body := split[0], split[1:]

	// Get the title
	titleMatch := titleRegex.FindStringSubmatch(header)
	if len(titleMatch) < 2 {
		fmt.Println("Could not match title for", item.Name())
		return postRequest, err
	}

	title := strings.TrimSpace(titleMatch[1])
	if title == "" {
		return postRequest, errors.New("Could not read title")
	}

	// Check the status
	draftMatch := draftRegex.FindStringSubmatch(header)

	status := "published"
	if len(draftMatch) > 1 && draftMatch[1] == "true" {
		status = "draft"
	}

	// Parse the date
	dateMatch := dateRegex.FindStringSubmatch(header)
	if len(dateMatch) < 2 {
		return postRequest, errors.New("Could not match date")
	}

	var t time.Time

	if strings.HasSuffix(dateMatch[1], "Z") {
		t, err = time.Parse(time.RFC3339, dateMatch[1])
	} else if strings.HasSuffix(dateMatch[1], "CET") {
		t, err = time.Parse("2006-01-02 15:04:05 -0700 CET", dateMatch[1])
	} else if strings.HasSuffix(dateMatch[1], "CEST") {
		t, err = time.Parse("2006-01-02 15:04:05 -0700 CEST", dateMatch[1])
	} else {
		t, err = time.Parse("2006-01-02T15:04:05-07:00", dateMatch[1])
	}

	if err != nil || t.UnixMilli() < 1000 {
		return postRequest, errors.New("Could not parse time")
	}

	// Escape title and content
	titleMarshal, err := json.Marshal(title)
	if err != nil {
		return postRequest, errors.New("Could not marshal title")
	}

	title = string(titleMarshal)

	contentMarshal, err := json.Marshal(strings.TrimSpace(strings.Join(body, "+++")))
	if err != nil {
		return postRequest, errors.New("Could not marshal content")
	}

	content := string(contentMarshal)

	postRequest.title = title[1 : len(title)-1]
	postRequest.content = content[1 : len(content)-1]
	postRequest.status = status
	postRequest.date = fmt.Sprint(t.UnixMilli())
	postRequest.url = generateUrl(t, postRequest.title, postRequest.filename)

	return postRequest, nil
}

func main() {
	items, err := os.ReadDir(DIRECTORY)
	if err != nil {
		fmt.Println("Could not read directory", DIRECTORY)
		os.Exit(1)
	}

	hasError := false

	var postRequests []PostRequest

	for _, item := range items {
		postRequest, err := validateItem(item)
		if err != nil {
			hasError = true
			fmt.Println("Error in file", postRequest.filename, err)
			continue
		}

		postRequests = append(postRequests, postRequest)
	}

	if hasError {
		os.Exit(1)
	}

	for _, postRequest := range postRequests {
		request, err := http.NewRequest("POST", ENDPOINT, bytes.NewBuffer([]byte(`{
			"title": "`+postRequest.title+`",
			"status": "`+postRequest.status+`",
			"date": `+postRequest.date+`,
			"url": "`+postRequest.url+`"
		}`)))

		if err != nil {
			fmt.Println("Could not create request for", postRequest.filename)
			continue
		}

		request.Header.Set("Content-Type", "application/json; charset=utf-8")

		client := &http.Client{}
		response, err := client.Do(request)
		if err == nil {
			continue
		}

		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println("Could not POST for", postRequest.filename)
			continue
		}

		formattedData, err := formatJSON(responseBody)
		if err != nil {
			fmt.Println("Could not parse response body for", postRequest.filename)
			continue
		}

		fmt.Println("Response for", postRequest.filename, formattedData)
	}
}
