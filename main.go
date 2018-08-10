package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

var credsFile string
var tokenFile string

func init() {
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

	return fmt.Sprintf("[%s] %s", fdate, event.Summary)
}

var rootCmd = &cobra.Command{
	Use:   "gocalendar-agenda-cli-tool",
	Short: "Simple cli tool to display your upcoming agenda",
	Run: func(cmd *cobra.Command, args []string) {
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

		t := time.Now().Format(time.RFC3339)
		events, err := srv.Events.List("primary").ShowDeleted(false).
			SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
		if err != nil {
			log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
		}

		if len(events.Items) == 0 {
			fmt.Println("No events for today.")
		} else {
			for i, item := range events.Items {

				fmt.Print(formatEvent(item))

				if i >= 1 {
					fmt.Print("\n")
					return
				} else {
					fmt.Print(" -> ")
				}
			}
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
