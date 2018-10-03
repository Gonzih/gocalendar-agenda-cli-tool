package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var credsFile string
var tokenFile string

func init() {
	rootCmd.AddCommand(agendaCmd, zoomCmd)
	rootCmd.PersistentFlags().StringVarP(&credsFile, "credentials", "", "credentials.json", "credentials file (default is credentials.json)")
	rootCmd.PersistentFlags().StringVarP(&tokenFile, "token", "", "token.json", "token file (default is token.json)")
}

func formatEvent(event *calendar.Event) string {
	dateS := event.Start.DateTime
	if dateS == "" {
		dateS = event.Start.Date
	}

	date, err := time.Parse("2006-01-02T15:04:05-07:00", dateS)
	if err != nil {
		log.Panicf("Could not parse a date %s %s", dateS, err)
	}

	fdate := date.Format("15:04")
	summary := event.Summary
	limit := 50

	if len(summary) > limit {
		summary = fmt.Sprintf("%s...", strings.Trim(summary[:limit], " \n\r\"'"))

	}

	return fmt.Sprintf("[%s] %s", fdate, summary)
}

func getEvents() *calendar.Events {
	b, err := ioutil.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now()
	startTime := t.Format(time.RFC3339)
	endTime := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 59, t.Location()).Format(time.RFC3339)

	events, err := srv.Events.
		List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(startTime).
		TimeMax(endTime).
		MaxResults(10).
		OrderBy("startTime").
		Do()

	must(err)

	return events
}

var rootCmd = &cobra.Command{
	Use:   "gocalendar-agenda-cli-tool",
	Short: "Simple cli tool to display your upcoming agenda",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var agendaCmd = &cobra.Command{
	Use:   "agenda",
	Short: "Print next 2 upcoming meetings",
	Run: func(cmd *cobra.Command, args []string) {
		events := getEvents()

		if len(events.Items) == 0 {
			fmt.Println("No events for today.")
		} else {
			fmt.Print(formatEvent(events.Items[0]))

			if len(events.Items) > 1 {
				fmt.Print(" -> ")
				fmt.Print(formatEvent(events.Items[1]))
			}

			fmt.Print("\n")
		}
	},
}

var linkRe = regexp.MustCompile(`https://([^\.]+\.)?zoom\.us/j/\d*`)

func getZoomLink(e *calendar.Event) string {
	return linkRe.FindString(e.Description)
}

var zoomCmd = &cobra.Command{
	Use:   "zoom",
	Short: "Print next upcoming meeting's zoom link",
	Run: func(cmd *cobra.Command, args []string) {
		events := getEvents()
		if len(events.Items) > 0 {
			fmt.Print(getZoomLink(events.Items[0]))
		}
	},
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}
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

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
