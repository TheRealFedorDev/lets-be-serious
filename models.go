package main

type User struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type Preferences struct {
	Accent   string `json:"accent"`
	MenuTint string `json:"menuTint"`
}

type Destination struct {
	ID          int64    `json:"id"`
	Slug        string   `json:"slug"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	Image       string   `json:"image"`
	PriceFrom   int      `json:"priceFrom"`
	Tags        []string `json:"tags"`
}

type HotelCard struct {
	ID               int64    `json:"id"`
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	City             string   `json:"city"`
	Country          string   `json:"country"`
	DestinationSlug  string   `json:"destinationSlug"`
	Image            string   `json:"image"`
	Address          string   `json:"address"`
	ShortDescription string   `json:"shortDescription"`
	Rating           float64  `json:"rating"`
	Reviews          int      `json:"reviews"`
	PriceFrom        int      `json:"priceFrom"`
	Tags             []string `json:"tags"`
	Featured         bool     `json:"featured"`
	Favorited        bool     `json:"favorited"`
}

type Room struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int      `json:"price"`
	Capacity    int      `json:"capacity"`
	Bed         string   `json:"bed"`
	Size        int      `json:"size"`
	Amenities   []string `json:"amenities"`
	Image       string   `json:"image"`
	TotalUnits  int      `json:"-"`
	Available   int      `json:"available"`
}

type POI struct {
	ID          int64    `json:"id,omitempty"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	Distance    string   `json:"distance"`
	Tags        []string `json:"tags"`
	PriceLevel  int      `json:"priceLevel"`
}

type HotelDetail struct {
	HotelCard
	Description string      `json:"description"`
	Gallery     []string    `json:"gallery"`
	Amenities   []string    `json:"amenities"`
	Destination Destination `json:"destination"`
	Rooms       []Room      `json:"rooms"`
	Nearby      []POI       `json:"nearby"`
	CheckIn     string      `json:"checkIn"`
	CheckOut    string      `json:"checkOut"`
	Guests      int         `json:"guests"`
}

type Trip struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Status     string `json:"status"`
	HotelName  string `json:"hotelName"`
	HotelSlug  string `json:"hotelSlug"`
	City       string `json:"city"`
	Image      string `json:"image"`
	RoomName   string `json:"roomName"`
	CheckIn    string `json:"checkIn"`
	CheckOut   string `json:"checkOut"`
	Nights     int    `json:"nights"`
	Guests     int    `json:"guests"`
	Total      int    `json:"total"`
	GuestName  string `json:"guestName"`
	GuestEmail string `json:"guestEmail"`
	GuestPhone string `json:"guestPhone"`
	CanCancel  bool   `json:"canCancel"`
	CreatedAt  string `json:"createdAt"`
}

type HomeStats struct {
	HotelCount       int     `json:"hotelCount"`
	DestinationCount int     `json:"destinationCount"`
	AverageRating    float64 `json:"averageRating"`
	ConciergeCount   int     `json:"conciergeCount"`
}

type HomePayload struct {
	Destinations []Destination `json:"destinations"`
	Featured     []HotelCard   `json:"featured"`
	Stats        HomeStats     `json:"stats"`
}

type BookingInput struct {
	HotelID    int64  `json:"hotelId"`
	RoomID     int64  `json:"roomId"`
	CheckIn    string `json:"checkIn"`
	CheckOut   string `json:"checkOut"`
	Guests     int    `json:"guests"`
	GuestName  string `json:"guestName"`
	GuestEmail string `json:"guestEmail"`
	GuestPhone string `json:"guestPhone"`
}

type AssistantHotel struct {
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	City      string   `json:"city"`
	Image     string   `json:"image"`
	PriceFrom int      `json:"priceFrom"`
	Rating    float64  `json:"rating"`
	Tags      []string `json:"tags"`
}

type AssistantResponse struct {
	Reply       string           `json:"reply"`
	Source      string           `json:"source"`
	Hotels      []AssistantHotel `json:"hotels"`
	Places      []POI            `json:"places"`
	Destination string           `json:"destination,omitempty"`
}

type SearchResult struct {
	Hotels []HotelCard `json:"hotels"`
	Total  int         `json:"total"`
	Query  string      `json:"query"`
}

type SearchParams struct {
	Query       string
	Destination string
	Guests      int
	MaxPrice    int
	Tag         string
	Sort        string
}

type AuthPayload struct {
	User        User        `json:"user"`
	Preferences Preferences `json:"preferences"`
}
