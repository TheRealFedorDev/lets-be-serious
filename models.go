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

type PriceBreakdown struct {
	Room      int `json:"room"`
	Taxes     int `json:"taxes"`
	Fees      int `json:"fees"`
	Breakfast int `json:"breakfast"`
	Total     int `json:"total"`
}

type WeatherInfo struct {
	Condition   string   `json:"condition"`
	Description string   `json:"description"`
	TempC       int      `json:"tempC"`
	Icon        string   `json:"icon"`
	Tags        []string `json:"tags"`
}

type HotelCard struct {
	ID               int64          `json:"id"`
	Slug             string         `json:"slug"`
	Name             string         `json:"name"`
	City             string         `json:"city"`
	Country          string         `json:"country"`
	DestinationSlug  string         `json:"destinationSlug"`
	Image            string         `json:"image"`
	Address          string         `json:"address"`
	ShortDescription string         `json:"shortDescription"`
	Rating           float64        `json:"rating"`
	Reviews          int            `json:"reviews"`
	PriceFrom        int            `json:"priceFrom"`
	PriceBreakdown   PriceBreakdown `json:"priceBreakdown"`
	Tags             []string       `json:"tags"`
	TagSlugs         []string       `json:"tagSlugs"`
	Featured         bool           `json:"featured"`
	Favorited        bool           `json:"favorited"`
	Meta             HotelMeta      `json:"meta"`
	Match            int            `json:"match,omitempty"`
	DistanceKm       float64        `json:"distanceKm,omitempty"`
	Capacity         int            `json:"capacity,omitempty"`
	Amenities        []string       `json:"amenities,omitempty"`
	Needs            []NeedCheck    `json:"needs,omitempty"`
}

type Room struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Price          int            `json:"price"`
	PriceBreakdown PriceBreakdown `json:"priceBreakdown"`
	Capacity       int            `json:"capacity"`
	Bed            string         `json:"bed"`
	Size           int            `json:"size"`
	Amenities      []string       `json:"amenities"`
	Image          string         `json:"image"`
	TotalUnits     int            `json:"-"`
	Available      int            `json:"available"`
	BedTypes       []string       `json:"bedTypes,omitempty"`
	View           string         `json:"view,omitempty"`
	Kitchen        bool           `json:"kitchen,omitempty"`
	KidsCot        bool           `json:"kidsCot,omitempty"`
}

type POI struct {
	ID              int64    `json:"id,omitempty"`
	Name            string   `json:"name"`
	Category        string   `json:"category"`
	Description     string   `json:"description"`
	Address         string   `json:"address"`
	Distance        string   `json:"distance"`
	Minutes         int      `json:"minutes,omitempty"`
	Tags            []string `json:"tags"`
	PriceLevel      int      `json:"priceLevel"`
	DestinationSlug string   `json:"destinationSlug,omitempty"`
	Lat             float64  `json:"lat,omitempty"`
	Lng             float64  `json:"lng,omitempty"`
	Schematic       bool     `json:"schematic,omitempty"`
}

type HotelDetail struct {
	HotelCard
	Description string          `json:"description"`
	Gallery     []string        `json:"gallery"`
	Amenities   []string        `json:"amenities"`
	Destination Destination     `json:"destination"`
	Rooms       []Room          `json:"rooms"`
	Nearby      []POI           `json:"nearby"`
	Excursions  []POI           `json:"excursions"`
	CheckIn     string          `json:"checkIn"`
	CheckOut    string          `json:"checkOut"`
	Guests      int             `json:"guests"`
	MatchDetail *MatchBreakdown `json:"matchDetail,omitempty"`
	Walk        *WalkingRoute   `json:"walk,omitempty"`
}

type POIDetail struct {
	POI    POI         `json:"poi"`
	Hotels []HotelCard `json:"hotels"`
	City   string      `json:"city"`
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
	// Оплаченные вместе с номером позиции программы (order.go).
	Items      []OrderItem `json:"items,omitempty"`
	OrderTotal int         `json:"orderTotal,omitempty"`
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
	Deals        []HotelCard   `json:"deals"`
	Moods        []MoodDef     `json:"moods"`
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

type AssistantRoom struct {
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Capacity int    `json:"capacity,omitempty"`
	Bed      string `json:"bed,omitempty"`
	Size     int    `json:"size,omitempty"`
}

type AssistantHotel struct {
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	City             string          `json:"city"`
	Image            string          `json:"image"`
	PriceFrom        int             `json:"priceFrom"`
	Rating           float64         `json:"rating"`
	Reviews          int             `json:"reviews,omitempty"`
	Tags             []string        `json:"tags"`
	Address          string          `json:"address,omitempty"`
	Amenities        []string        `json:"amenities,omitempty"`
	ShortDescription string          `json:"shortDescription,omitempty"`
	Description      string          `json:"description,omitempty"`
	Rooms            []AssistantRoom `json:"rooms,omitempty"`
	Nearby           []string        `json:"nearby,omitempty"`
	Distances        []string        `json:"distances,omitempty"`
	PriceTotal       int             `json:"priceTotal,omitempty"`
	Highlight        string          `json:"highlight,omitempty"`
	Stars            int             `json:"stars,omitempty"`
}

type AssistantResponse struct {
	Reply       string            `json:"reply"`
	Source      string            `json:"source"`
	Hotels      []AssistantHotel  `json:"hotels"`
	Places      []POI             `json:"places"`
	Destination string            `json:"destination,omitempty"`
	Focus       *AssistantHotel   `json:"focus,omitempty"`
	Actions     []AssistantAction `json:"actions,omitempty"`
}

type SearchResult struct {
	Hotels []HotelCard  `json:"hotels"`
	Total  int          `json:"total"`
	Query  string       `json:"query"`
	Facets []FacetGroup `json:"facets"`
	Mood   string       `json:"mood,omitempty"`
	Tags   []string     `json:"tags,omitempty"`
	Nearby bool         `json:"nearby,omitempty"`
	Needs  NeedsEval    `json:"needsEval"`
}

type SearchParams struct {
	Query       string
	Destination string
	Guests      int
	MaxPrice    int
	MinPrice    int
	MinRating   float64
	Tag         string
	Tags        []string
	Mood        string
	Sort        string
	Insured     bool
	Guides      bool
	Parking     bool
	Pool        bool
	Soundproof  bool
	Week        bool
	Discount    bool
	Lat         float64
	Lng         float64
	RadiusKm    float64
	CheckIn     string
	CheckOut    string
	Needs       []NeedSpec
}

type AuthPayload struct {
	User        User        `json:"user"`
	Preferences Preferences `json:"preferences"`
}
