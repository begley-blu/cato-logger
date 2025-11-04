package api

// Request represents a GraphQL API request
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// EventsFeedResponse represents the API response structure
type EventsFeedResponse struct {
	Data struct {
		EventsFeed struct {
			Marker       *string `json:"marker"`
			FetchedCount int     `json:"fetchedCount"`
			Accounts     []struct {
				ID          string `json:"id"`
				ErrorString string `json:"errorString"`
				Records     []struct {
					FieldsMap map[string]string `json:"fieldsMap"`
				} `json:"records"`
			} `json:"accounts"`
		} `json:"eventsFeed"`
	} `json:"data"`
	Errors []struct {
		Message   string `json:"message"`
		Locations []struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"locations,omitempty"`
		Path       []string               `json:"path,omitempty"`
		Extensions map[string]interface{} `json:"extensions,omitempty"`
	} `json:"errors,omitempty"`
}

// EventsPage represents a page of events from the API
type EventsPage struct {
	Events    []map[string]string
	NewMarker string
	HasMore   bool
}
