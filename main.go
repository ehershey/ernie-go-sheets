package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	debug("Got a token")
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Fprintf(os.Stderr, "Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		log.Fatalf("Unable to encode token as json: %v", err)
	}
}

var debugEnabled *bool
var format *string 

func debug(msg string, args ...any) {
	if *debugEnabled {
		fmt.Fprintf(os.Stderr, msg, args...)
		fmt.Fprintf(os.Stderr, "\n")
	}
}

func main() {
	debugEnabled = flag.Bool("debug", false, "Print debugging msgs to stderr")
	format = flag.String("format", "text", "Output format (text or json)")
	flag.Parse()
	debug("Debug logging enabled")

	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	debug("Read credentials.json (didn't parse)")

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	debug("Parsed credentials.json")
	client := getClient(config)
	debug("Got client from auth config")

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}
	debug("Got service object from client")

	spreadsheetId := os.Getenv("MARATHON_SPREADSHEET_ID")
	readRange := "Main"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return
	}
	myNotesCol := 0
	foundMyNotesCol := false
	planNotesCol := 0
	foundPlanNotesCol := false
	dateCol := 0
	foundDateCol := false
	distanceCol := 0
	foundDistanceCol := false
	plannedDistance := 0.0
	myNotes := ""
	planNotes := ""
	foundMyNotes := false
	foundPlanNotes := false
	foundPlannedDistance := false
	today := time.Now().Format("01/02")
	today_alt := time.Now().Format("1/2")
	debug("Everything:")

	var colNames = make([]string, 0)
	var todayRow []string
	for rowIndex, row := range resp.Values {
		rowOfStrings := make([]string, 0)
		// Print columns A and E, which correspond to indices 0 and 4.
		for colIndex, cellValue := range row {
			rowOfStrings = append(rowOfStrings, cellValue.(string))
			cellValue = strings.ReplaceAll(cellValue.(string), "\n", " ")
			cellValueLower := strings.ToLower(cellValue.(string))
			if rowIndex == 0 {
				colNames = append(colNames, cellValue.(string))
				// fmt.Printf("Comparing >%v< and >%v<\n", cellValue, "Date")
				// fmt.Printf("Comparing >%v< and >%v<\n", cellValue, "Distance Planned")
				// fmt.Printf("Comparing >%v< and >%v<\n", cellValue, "My Notes")
				// fmt.Printf("Comparing >%v< and >%v<\n", cellValue, "Plan Notes")
				if cellValueLower == strings.ToLower("Date") {
					dateCol = colIndex
					foundDateCol = true
				} else if cellValueLower == strings.ToLower("Distance Planned") {
					distanceCol = colIndex
					foundDistanceCol = true
				} else if cellValueLower == strings.ToLower("Plan Notes") {
					planNotesCol = colIndex
					foundPlanNotesCol = true
				} else if cellValueLower == strings.ToLower("My Notes") {
					myNotesCol = colIndex
					foundMyNotesCol = true
				}
			}

			if colIndex > 0 {
				debug(", ")
			}
			debug("%v", cellValue)

		}
		if foundDateCol && foundDistanceCol {
			//fmt.Printf("Comparing >%v< and >%v<\n", row[dateCol].(string), today)
			if row[dateCol].(string) == today || row[dateCol].(string) == today_alt {
				todayRow = rowOfStrings
				plannedDistance, err = strconv.ParseFloat(row[distanceCol].(string), 64)
				if err != nil {
					log.Fatalf("Error parsing planned Distance (%v), %v", row[distanceCol], err)
				}
				foundPlannedDistance = true

				if foundMyNotesCol {
					myNotes = row[myNotesCol].(string)
					foundMyNotes = true
				}
				if foundPlanNotesCol {
					planNotes = row[planNotesCol].(string)
					foundPlanNotes = true
				}
			}
		} else {
			debug("foundDateCol: %v\n", foundDateCol)
			debug("foundDistanceCol: %v\n", foundDistanceCol)
			log.Fatalf("Did not find either date or distance column")
		}
		debug("\n")
		//fmt.Printf("%s, %s\n", row[0], row[1], row[2], row[3], row[4])
	}
	summaryMode := false
	fullRowMode := true
	if summaryMode {
		if foundPlannedDistance {
			fmt.Printf("Planned Distance today (%v): %v mi\n", today, plannedDistance)
			if foundPlanNotes {
				fmt.Printf("Plan Notes:\n%v\n", planNotes)
			}
			if foundMyNotes {
				fmt.Printf("My Notes:\n%v\n", myNotes)
			}
		} else {
			fmt.Printf("Did not find planned distance for today (%v)!\n", today)
		}
	}

	if fullRowMode {
		debug("len(colNames): %v", len(colNames))
		debug("len(todayRow): %v", len(todayRow))
		debug("colNames: %v", colNames)
		debug("todayRow: %v", todayRow)
		if *format == "json" { fmt.Printf("{\n") }
		for index, field := range colNames {
			if index < len(todayRow) {
		if *format == "json" { 
		fmt.Printf("\"%s\": \"%v\"\n", field, todayRow[index]) 
		
			if index < len(todayRow) -1 {
			fmt.Printf(",")
				}
		} else {
				fmt.Printf("%s: %v\n", field, todayRow[index])
			}
		}
	}
		if *format == "json" { fmt.Printf("}\n") }
	}
}
