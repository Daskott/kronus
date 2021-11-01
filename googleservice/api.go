package googleservice

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"log"
	"net/http"
	"os"

	"github.com/Daskott/kronus/types"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GCalendarAPIInterface interface {
	// CreateEvents creates google calendar events for the given contacts and returns the eventIDs and error(if any)
	CreateEvents(
		groupContacts []types.Contact,
		slotStartTime,
		slotEndTime,
		eventRecurrence string) ([]string, error)

	// CreateEvent creates a google calendar event and returns the event ID
	CreateEvent(contact, startTime, endTime, recurrence string) (string, error)

	// ClearAllEvents deletes all google calendar events for eventIDs
	ClearAllEvents(eventIDs []string) error
}

type GCalendarAPI struct {
	service *calendar.Service
}

const calendarId = "primary"

func NewGoogleCalendarAPI(credentials types.GoogleAppCredentials) *GCalendarAPI {
	ctx := context.Background()
	b, err := json.Marshal(credentials)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved *'token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	calendarService, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	return &GCalendarAPI{service: calendarService}
}

func (gcalAPI GCalendarAPI) CreateEvents(
	groupContacts []types.Contact,
	slotStartTime,
	slotEndTime,
	eventRecurrence string) ([]string, error) {

	startDate := time.Now()
	endDate := startDate.Add(30 * time.Minute)
	startStr, endStr := "", ""
	eventIds := []string{}
	for _, user := range groupContacts {
		startStr = fmt.Sprintf("%sT%s:00", startDate.Format("2006-01-02"), slotStartTime)
		endStr = fmt.Sprintf("%sT%s:00", endDate.Format("2006-01-02"), slotEndTime)

		eventId, err := gcalAPI.CreateEvent(user.Name, startStr, endStr, eventRecurrence)
		if err != nil {
			// Create all events or no event
			delErr := gcalAPI.ClearAllEvents(eventIds)
			if delErr != nil {
				err = fmt.Errorf("%v; %v", err, delErr)
			}
			return nil, fmt.Errorf("unable to create event. %v", err)
		}
		eventIds = append(eventIds, eventId)

		// Move date range to the next day for the next user
		startDate = startDate.Add(24 * time.Hour)
		endDate = startDate.Add(30 * time.Minute)
	}
	return eventIds, nil
}

func (gcalAPI GCalendarAPI) CreateEvent(contact, startTime, endTime, recurrence string) (string, error) {
	event := &calendar.Event{
		Summary:     fmt.Sprintf("☕ Coffee chat with %s", contact),
		Location:    "",
		Description: "A quick sync :)",
		Start: &calendar.EventDateTime{
			DateTime: startTime,
			TimeZone: "America/Toronto",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime,
			TimeZone: "America/Toronto",
		},
		Recurrence: []string{recurrence},
		Reminders: &calendar.EventReminders{
			Overrides: []*calendar.EventReminder{
				{
					Method:  "popup",
					Minutes: 10,
				},
			},
			ForceSendFields: []string{"UseDefault"},
		},
		Attendees: []*calendar.EventAttendee{},
	}

	event, err := gcalAPI.service.Events.Insert(calendarId, event).Do()
	if err != nil {
		return "", err
	}

	return event.Id, nil
}

func (gcalAPI GCalendarAPI) ClearAllEvents(eventIDs []string) error {
	var err error
	errorMsg := ""

	for _, eventID := range eventIDs {
		err = gcalAPI.service.Events.Delete(calendarId, eventID).Do()
		if err != nil {
			errorMsg += fmt.Sprintf("unable to delete event = %v because %v;", eventID, err)
		}
	}

	if errorMsg != "" {
		err = fmt.Errorf(errorMsg)
	}

	return err
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file kronus-token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	// Read token from file
	tokFileFilePath := filepath.Join(home, ".kronus-token.json")
	token, err := tokenFromFile(tokFileFilePath)

	// Get updated token by calling TokenSource(auto-renews token if expired) with token from file.
	// Doing this because, we'd like to prompt the user to sign-in to their google account
	// when their original token in file can no longer be renewed.
	if err == nil {
		token, err = config.TokenSource(context.TODO(), token).Token()
	}

	if err != nil || !token.Valid() {
		token = getTokenFromWeb(config)
		saveToken(tokFileFilePath, token)
	}

	return config.Client(context.Background(), token)
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
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
