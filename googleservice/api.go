package googleservice

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GCalendarAPIInterface interface {
	// CreateEvents creates google calendar events for the given contacts and returns the eventIDs and error(if any)
	CreateEvents(
		groupContacts []Contact,
		slotStartTime,
		slotEndTime,
		eventRecurrence string) ([]string, error)

	// CreateEvent creates a google calendar event and returns the event ID
	CreateEvent(contact, startTime, endTime, recurrence string) (string, error)

	// ClearAllEvents deletes all google calendar events for eventIDs
	ClearAllEvents(eventIDs []string) error
}

type GCalendarAPI struct {
	service    *calendar.Service
	calendarId string // The calendar to update
}

type Contact struct {
	Name string
}

func NewGoogleCalendarAPI(credentialsPath, calendarEmail string) (*GCalendarAPI, error) {
	ctx := context.Background()
	calendarService, err := calendar.NewService(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Google Calendar client: %v", err)
	}

	return &GCalendarAPI{service: calendarService, calendarId: calendarEmail}, nil
}

func (gcalAPI GCalendarAPI) CreateEvents(
	groupContacts []Contact,
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

	event, err := gcalAPI.service.Events.Insert(gcalAPI.calendarId, event).Do()
	if err != nil {
		return "", err
	}

	return event.Id, nil
}

func (gcalAPI GCalendarAPI) ClearAllEvents(eventIDs []string) error {
	var err error
	errorMsg := ""

	for _, eventID := range eventIDs {
		err = gcalAPI.service.Events.Delete(gcalAPI.calendarId, eventID).Do()
		if err != nil {
			errorMsg += fmt.Sprintf("unable to delete event = %v because %v;", eventID, err)
		}
	}

	if errorMsg != "" {
		err = fmt.Errorf(errorMsg)
	}

	return err
}
