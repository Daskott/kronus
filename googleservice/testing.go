package googleservice

type GCalendarAPIStub struct {
	CreatedEventsID     []string
	CreatedEventID      string
	CreatedEventsError  error
	CreatedEventError   error
	ClearAllEventsError error
}

func (gcalAPI GCalendarAPIStub) CreateEvents(
	groupContacts []Contact,
	slotStartTime,
	slotEndTime,
	eventRecurrence string) ([]string, error) {

	return gcalAPI.CreatedEventsID, gcalAPI.CreatedEventsError
}

func (gcalAPI GCalendarAPIStub) CreateEvent(contact, startTime, endTime, recurrence string) (string, error) {
	return gcalAPI.CreatedEventID, gcalAPI.CreatedEventError
}

func (gcalAPI GCalendarAPIStub) ClearAllEvents(eventIDs []string) error {
	return gcalAPI.ClearAllEventsError
}
