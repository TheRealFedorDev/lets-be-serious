package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseNeedsAppliesDefaultsAndLevels(t *testing.T) {
	specs := ParseNeeds([]string{"beach:10:2,metro:1", "pool"})
	if len(specs) != 3 {
		t.Fatalf("expected 3 needs, got %d", len(specs))
	}
	if specs[0].Key != "beach" || specs[0].Level != NeedLevelMust || specs[0].MaxMinutes != 10 {
		t.Fatalf("unexpected beach spec: %+v", specs[0])
	}
	if specs[1].Key != "metro" || specs[1].Level != NeedLevelSoft || specs[1].MaxMinutes != 1 {
		t.Fatalf("unexpected metro spec: %+v", specs[1])
	}
	if specs[2].Key != "pool" || specs[2].Kind != NeedKindHotel {
		t.Fatalf("unexpected pool spec: %+v", specs[2])
	}
}

func TestParseNeedsKeepsStrictestLevel(t *testing.T) {
	specs := ParseNeeds([]string{"beach:10:1", "beach:10:2"})
	if len(specs) != 1 {
		t.Fatalf("expected dedup to one need, got %d", len(specs))
	}
	if specs[0].Level != NeedLevelMust {
		t.Fatalf("expected mandatory level, got %d", specs[0].Level)
	}
}

func TestEvaluateNeedsMissingDataNeverRejects(t *testing.T) {
	hotel := HotelCard{Slug: "unknown", Meta: HotelMeta{}}
	specs := ParseNeeds([]string{"beach:5:2"})
	checks, ok, reason := EvaluateNeeds(hotel, specs)
	if !ok || reason != "" {
		t.Fatalf("missing distance should not reject, reason=%q", reason)
	}
	if len(checks) != 1 || checks[0].Known || !checks[0].OK {
		t.Fatalf("expected unknown-but-ok check, got %+v", checks)
	}
}

func TestEvaluateNeedsHardRejectsMandatory(t *testing.T) {
	hotel := HotelCard{Slug: "far", Meta: HotelMeta{BeachMinutes: 20}}
	checks, ok, reason := EvaluateNeeds(hotel, ParseNeeds([]string{"beach:5:2"}))
	if ok {
		t.Fatalf("expected rejection, got checks %+v", checks)
	}
	if reason == "" {
		t.Fatal("expected human-readable rejection reason")
	}
}

func TestEvaluateNeedsSoftNeverRejects(t *testing.T) {
	hotel := HotelCard{Slug: "far", Meta: HotelMeta{BeachMinutes: 20}}
	_, ok, reason := EvaluateNeeds(hotel, ParseNeeds([]string{"beach:5:1"}))
	if !ok || reason != "" {
		t.Fatalf("soft need should not reject, reason=%q", reason)
	}
}

func TestMatchFactorPointsSumToScore(t *testing.T) {
	cases := []HotelCard{
		{Slug: "sunny", Rating: 9.3, Reviews: 800, PriceFrom: 12000, Meta: HotelMeta{
			Insured: true, Guides: true, Pool: true, Parking: true, Soundproof: true,
			BeachMinutes: 3, MetroMinutes: 6, FoodMinutes: 4,
			TagSlugs: []string{"plyazh", "zavtraki", "romantika"},
		}},
		{Slug: "plain", Rating: 7.1, PriceFrom: 4000},
	}
	params := SearchParams{Guests: 2, Mood: "romantika", Tags: []string{"plyazh"}, MaxPrice: 10000}
	for _, hotel := range cases {
		breakdown := BuildMatchBreakdown(hotel, params, "romantika", []string{"plyazh"})
		sum := 0
		for _, factor := range breakdown.Factors {
			sum += factor.Points
		}
		if sum != breakdown.Score {
			t.Fatalf("%s: factor points sum %d != score %d", hotel.Slug, sum, breakdown.Score)
		}
		if breakdown.Score < 0 || breakdown.Score > 100 {
			t.Fatalf("%s: score out of range: %d", hotel.Slug, breakdown.Score)
		}
	}
}

func TestTripBudgetCountsStayAndExtras(t *testing.T) {
	hotel := HotelCard{PriceFrom: 10000, PriceBreakdown: PriceBreakdown{Total: 10000}}
	req := TripRequest{Guests: 2, Budget: 100000}
	days := []TripDay{{
		Blocks: []TripBlock{
			{Type: "food", Price: 2000},
			{Type: "activity", Price: 3000},
			{Type: "rest", Price: 500},
			{Type: "transfer", Price: 0},
		},
	}}
	budget := buildTripBudget(hotel, req, 2, days)
	if budget.Split["stay"] != 20000 {
		t.Fatalf("expected stay 20000, got %d", budget.Split["stay"])
	}
	if budget.Split["food"] != 2000 || budget.Split["activities"] != 3500 {
		t.Fatalf("unexpected food/activities split: %+v", budget.Split)
	}
	if budget.Total != 20000+2000+3500+700*2*2 {
		t.Fatalf("unexpected total: %d", budget.Total)
	}
	if !budget.Fits {
		t.Fatal("expected trip to fit the budget")
	}
}

func TestWhatIfBudgetLimitMatchesLabel(t *testing.T) {
	if got := budgetLimit(0, 21400); got != 25680 {
		t.Fatalf("no budget: expected 25680 (+20%% of 21400), got %d", got)
	}
	if got := budgetLimit(20000, 21400); got != 24000 {
		t.Fatalf("budget 20000: expected 24000 (+20%%), got %d", got)
	}
}

func TestWhatIfValidateRejectsOutOfRange(t *testing.T) {
	ok := WhatIfRequest{Slug: "x", Guests: 2, Nights: 1, Budget: 0}
	if err := validateWhatIf(ok); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	bad := []WhatIfRequest{
		{Slug: "x", Guests: 2, Nights: 1 << 62},
		{Slug: "x", Guests: 2, Nights: 0},
		{Slug: "x", Guests: 2, Nights: 31},
		{Slug: "x", Guests: 0, Nights: 1},
		{Slug: "x", Guests: 11, Nights: 1},
		{Slug: "x", Guests: 2, Nights: 1, Budget: -1},
		{Slug: "x", Guests: 2, Nights: 1, Budget: 10_000_001},
	}
	for _, req := range bad {
		err := validateWhatIf(req)
		if _, isInput := err.(*WhatIfInputError); !isInput || err.Error() == "" {
			t.Fatalf("expected WhatIfInputError for %+v, got %v", req, err)
		}
	}
}

func TestTripPaceChangesArrivalAndDepartureDays(t *testing.T) {
	destination := Destination{Slug: "sochi", City: "Сочи"}
	hotel := HotelCard{Name: "Тест", PriceFrom: 10000, PriceBreakdown: PriceBreakdown{Total: 10000, Breakfast: 900}}
	pois := []POI{
		{ID: 1, Name: "Парк", Category: "nature", Minutes: 60},
		{ID: 2, Name: "Кафе", Category: "food", Minutes: 60},
		{ID: 3, Name: "Кафе 2", Category: "food", Minutes: 60},
	}
	sig := func(d TripDay) string {
		out := ""
		for _, b := range d.Blocks {
			out += b.Time + b.Title + b.Place + ";"
		}
		return out
	}
	build := func(pace string) []TripDay {
		return buildTripDays(destination, hotel, pois, TripRequest{From: "2026-11-01", Guests: 2, Pace: pace}, 3, &Store{})
	}
	calm, active := build("calm"), build("active")
	for _, i := range []int{0, len(calm) - 1} {
		if sig(calm[i]) == sig(active[i]) {
			t.Fatalf("day %d identical for calm and active", i+1)
		}
	}
	for _, d := range calm[1 : len(calm)] {
		for _, b := range d.Blocks {
			if b.Type == "food" && (b.Title == "Завтрак" || b.Title == "Завтрак и кофе") && b.Price != 0 {
				t.Fatalf("included breakfast must cost 0, got %d", b.Price)
			}
		}
	}
	if got := clampInt(100, 1, 100); got != 100 {
		t.Fatalf("score must allow 100, got %d", got)
	}
}

func TestRainyTripDaySkipsOpenAirCulture(t *testing.T) {
	destination := Destination{Slug: "sochi", City: "Сочи"}
	hotel := HotelCard{Name: "Тест", PriceFrom: 10000, PriceBreakdown: PriceBreakdown{Total: 10000}}
	pois := []POI{
		{ID: 1, Name: "Скайпарк", Category: "culture", Tags: []string{"активность", "горы"}, Minutes: 35},
		{ID: 2, Name: "Музей", Category: "culture", Tags: []string{"музей"}, Minutes: 60},
		{ID: 3, Name: "Спа-центр", Category: "spa", Minutes: 60},
		{ID: 4, Name: "Кафе", Category: "food", Minutes: 60},
	}
	rainy := 0
	for day := 0; day < 120; day++ {
		from := addDays("2026-10-01", day/2)
		pace := []string{"calm", "active"}[day%2]
		days := buildTripDays(destination, hotel, pois, TripRequest{From: from, Guests: 2, Pace: pace}, 3, &Store{})
		for _, d := range days {
			if d.Weather.Condition != "rain" {
				continue
			}
			rainy++
			for _, b := range d.Blocks {
				if b.Place == "Скайпарк" {
					t.Fatalf("rainy day %s contains open-air Скайпарк", d.Date)
				}
			}
		}
	}
	if rainy == 0 {
		t.Fatal("no rainy days generated, test is not exercising the rainy branch")
	}
}

func testRooms(meta HotelMeta) []Room {
	rooms := []Room{
		{ID: 1, Name: "Studio", Price: 10000, Capacity: 2, Size: 28, Available: 3},
		{ID: 2, Name: "Deluxe", Price: 13500, Capacity: 3, Size: 40, Available: 2},
		{ID: 3, Name: "Suite", Price: 19000, Capacity: 4, Size: 58, Available: 1},
	}
	annotateRooms(rooms, meta)
	return rooms
}

func TestPickRoomRespectsCapacityAndAvailability(t *testing.T) {
	rooms := testRooms(HotelMeta{})
	if room, _, ok := pickRoom(rooms, 2, RoomPrefs{}); !ok || room.ID != 1 {
		t.Fatalf("2 guests without prefs should get the cheapest Studio, got %+v ok=%v", room, ok)
	}
	if room, _, ok := pickRoom(rooms, 3, RoomPrefs{}); !ok || room.ID != 2 {
		t.Fatalf("3 guests must not get a 2-person Studio, got %+v", room)
	}
	rooms[2].Available = 0
	if _, _, ok := pickRoom(rooms, 4, RoomPrefs{}); ok {
		t.Fatal("sold-out Suite must not be picked for 4 guests")
	}
}

func TestPickRoomFollowsPreferences(t *testing.T) {
	sea := testRooms(HotelMeta{TagSlugs: []string{"more"}})
	room, why, ok := pickRoom(sea, 2, RoomPrefs{Bed: "twin", View: "sea"})
	if !ok || room.ID != 2 {
		t.Fatalf("twin + sea view should pick Deluxe (cheapest match), got %+v", room)
	}
	if len(why) != 3 || !strings.HasPrefix(why[1], "✓") || !strings.HasPrefix(why[2], "✓") {
		t.Fatalf("why should confirm preferences, got %v", why)
	}
	room, _, _ = pickRoom(sea, 2, RoomPrefs{Kitchen: true, KidsCot: true})
	if room.ID != 3 {
		t.Fatalf("kitchen + kids cot should pick Suite, got %+v", room)
	}
	city := testRooms(HotelMeta{})
	room, why, _ = pickRoom(city, 2, RoomPrefs{View: "sea"})
	if room.ID != 1 {
		t.Fatalf("unavailable view must fall back to cheapest fitting room, got %+v", room)
	}
	if !strings.HasPrefix(why[len(why)-1], "✗") {
		t.Fatalf("missing view must be explained honestly, got %v", why)
	}
}

func TestNormalizePrefsDropsUnknownValues(t *testing.T) {
	req := TripRequest{Weather: "Hurricane", Interests: []string{"museums", "MUSEUMS", "casino", "gastro"},
		Room: RoomPrefs{Bed: "bunk", View: "moon", MinSize: -5}, MaxWalkMin: 999}
	normalizePrefs(&req)
	if req.Weather != "" || req.Room.Bed != "" || req.Room.View != "" || req.Room.MinSize != 0 {
		t.Fatalf("unknown values must be dropped: %+v", req)
	}
	if len(req.Interests) != 2 || req.Interests[0] != "gastro" || req.Interests[1] != "museums" {
		t.Fatalf("interests must be deduped, filtered and sorted: %v", req.Interests)
	}
	if req.MaxWalkMin != maxWalkMinutes {
		t.Fatalf("maxWalkMin must be capped, got %d", req.MaxWalkMin)
	}
}

func TestRankPOIsPutsInterestsFirstAndFarPointsLast(t *testing.T) {
	hotel := HotelCard{Meta: HotelMeta{Lat: 43.58, Lng: 39.72}}
	pois := []POI{
		{ID: 1, Name: "Набережная", Category: "walk", Lat: 43.581, Lng: 39.721},
		{ID: 2, Name: "Краеведческий музей", Category: "culture", Lat: 43.585, Lng: 39.725},
		{ID: 3, Name: "Дальний музей истории", Category: "culture", Lat: 43.70, Lng: 39.90},
	}
	if got := rankPOIs(pois, hotel, TripRequest{}); got[0].ID != 1 || got[2].ID != 3 {
		t.Fatalf("without prefs the order must not change: %v", got)
	}
	got := rankPOIs(pois, hotel, TripRequest{Interests: []string{"museums"}})
	if got[0].Category != "culture" || got[2].ID != 1 {
		t.Fatalf("museums must come first: %+v", got)
	}
	got = rankPOIs(pois, hotel, TripRequest{Interests: []string{"museums"}, MaxWalkMin: 20})
	if got[0].ID != 2 || got[len(got)-1].ID != 3 {
		t.Fatalf("near museum first, far point (beyond walk limit) last: %+v", got)
	}
	if label := poiDistanceLabel(hotel, pois[2]); !strings.Contains(label, "такси") {
		t.Fatalf("far point should suggest a taxi, got %q", label)
	}
}

func TestWeatherPreferenceDrivesIndoorDaysAndDateHint(t *testing.T) {
	mild, rain, cold := WeatherInfo{Condition: "mild", TempC: 20}, WeatherInfo{Condition: "rain", TempC: 15}, WeatherInfo{Condition: "cold", TempC: 2}
	if indoorDay(mild, "") || !indoorDay(rain, "") || !indoorDay(cold, "warm") || indoorDay(cold, "snow") {
		t.Fatal("indoorDay must follow rain and the weather preference")
	}
	s := &Store{}
	hinted := 0
	for day := 0; day < 90; day++ {
		from := addDays("2026-11-01", day)
		hint := s.bestDates("Сочи", from, 3, "dry")
		if hint == nil {
			continue
		}
		hinted++
		fitNow := 0
		for i := 0; i <= 3; i++ {
			if weatherFits(s.Weather("Сочи", addDays(from, i)), "dry") {
				fitNow++
			}
		}
		if hint.Fit <= fitNow || fitNow*2 >= 4 || nightsBetween(hint.From, hint.To) != 3 {
			t.Fatalf("hint must improve a bad trip and keep its length: from=%s now=%d hint=%+v", from, fitNow, hint)
		}
	}
	if hinted == 0 {
		t.Fatal("no date hints generated, test is not exercising bestDates")
	}
	if s.bestDates("Сочи", "2026-11-01", 3, "") != nil {
		t.Fatal("no preference - no hint")
	}
}

func TestCheckoutChargesServerPricesAndCancelsTogether(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "checkout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := seedBaseData(ctx, store); err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateUser(ctx, "Тест", "checkout@test.local", "x")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Now().AddDate(0, 0, 20).Format("2006-01-02")
	req := TripRequest{Destination: "sochi", From: from, To: addDays(from, 3), Guests: 2, Pace: "active",
		Interests: []string{"museums"}, Room: RoomPrefs{View: "sea"}}
	plan, err := store.BuildTrip(ctx, user.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Room == nil {
		t.Fatal("plan must pick a room")
	}
	ids, itemsSum := []string{}, 0
	for _, day := range plan.Days {
		for _, block := range day.Blocks {
			if block.Bookable {
				ids = append(ids, block.ItemID)
				itemsSum += block.Price
			}
		}
	}
	if len(ids) == 0 {
		t.Fatal("active trip must have bookable paid activities")
	}
	req.HotelSlug = plan.Hotel.Slug
	input := CheckoutInput{Trip: req, RoomID: plan.Room.ID, ItemIDs: append(ids, ids[0]),
		GuestName: "Тест", GuestEmail: "checkout@test.local", GuestPhone: "+70000000000"}

	bad := input
	bad.ItemIDs = []string{"d1-999999-1100"}
	var itemErr *ItemInputError
	if _, err := store.Checkout(ctx, user.ID, bad); !errors.As(err, &itemErr) {
		t.Fatalf("unknown item must be rejected, got %v", err)
	}

	result, err := store.Checkout(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	stay := plan.Room.PriceBreakdown.Total * plan.Nights
	if result.Trip.Total != stay || result.Total != stay+itemsSum || len(result.Items) != len(ids) {
		t.Fatalf("total must be stay %d + items %d (deduped %d), got trip=%d total=%d items=%d",
			stay, itemsSum, len(ids), result.Trip.Total, result.Total, len(result.Items))
	}

	trips, err := store.Trips(ctx, user.ID)
	if err != nil || len(trips) != 1 || trips[0].OrderTotal != result.Total || len(trips[0].Items) != len(ids) {
		t.Fatalf("trips must show the paid program: %+v err=%v", trips, err)
	}
	if err := store.CancelBooking(ctx, user.ID, result.Trip.ID); err != nil {
		t.Fatal(err)
	}
	trips, _ = store.Trips(ctx, user.ID)
	for _, item := range trips[0].Items {
		if item.Status != "cancelled" {
			t.Fatalf("cancelling the booking must cancel paid items, got %+v", item)
		}
	}
}

func TestExtractTripPrefsFromPhrases(t *testing.T) {
	sochi := Destination{Slug: "sochi", City: "Сочи"}
	cases := []struct {
		text  string
		check func(p AssistantTripPrefs) bool
	}{
		{"Хотим в Сочи вдвоём, чтобы без дождя, вид на море и гастро", func(p AssistantTripPrefs) bool {
			return p.Weather == "dry" && p.Room.View == "sea" && p.Guests == 2 && containsString(p.Interests, "gastro")
		}},
		{"едем с детьми, нужен бассейн и пляж рядом, детская кроватка", func(p AssistantTripPrefs) bool {
			return containsString(p.Interests, "kids") && strings.Contains(p.Needs, "pool::2") && strings.Contains(p.Needs, "beach::1") && p.Room.KidsCot
		}},
		{"бюджет до 60 тысяч, хочу тепло", func(p AssistantTripPrefs) bool { return p.Budget == 60000 && p.Weather == "warm" }},
		{"на 100к на неделю, музеи и история", func(p AssistantTripPrefs) bool {
			return p.Budget == 100000 && containsString(p.Interests, "museums") && nightsBetween(p.From, p.To) == 7
		}},
		{"150 000 ₽ на троих, две кровати", func(p AssistantTripPrefs) bool { return p.Budget == 150000 && p.Room.Bed == "twin" }},
		{"хочу снег и лыжи", func(p AssistantTripPrefs) bool { return p.Weather == "snow" && containsString(p.Interests, "active") }},
		{"без жары, чтобы до мест пешком не дальше 15 мин", func(p AssistantTripPrefs) bool { return p.Weather == "cool" && p.MaxWalkMin == 15 }},
		{"спокойный отдых, вид на горы", func(p AssistantTripPrefs) bool { return p.Pace == "calm" && p.Room.View == "mountain" }},
		{"дорога 100 км — это далеко?", func(p AssistantTripPrefs) bool { return p.Budget == 0 }},
	}
	for _, c := range cases {
		p, _ := extractTripPrefs(c.text, sochi)
		if !c.check(p) {
			t.Errorf("%q parsed as %+v", c.text, p)
		}
	}
	if _, ok := extractTripPrefs("Привет! Как дела?", sochi); ok {
		t.Error("small talk must not produce trip prefs")
	}
	if _, ok := extractTripPrefs("хочу тепло", Destination{}); ok {
		t.Error("without a city the wizard cannot build a trip")
	}
}

func TestSurpriseRespectsCriteriaAndSeed(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "surprise.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := seedBaseData(ctx, store); err != nil {
		t.Fatal(err)
	}
	from := time.Now().AddDate(0, 0, 25).Format("2006-01-02")
	req := SurpriseRequest{TripRequest: TripRequest{From: from, To: addDays(from, 3), Guests: 2, Budget: 120000, Weather: "dry"}}
	seen := map[string]bool{}
	for seed := int64(1); seed <= 15; seed++ {
		req.Seed = seed
		a, err := store.Surprise(ctx, 0, req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := store.Surprise(ctx, 0, req)
		if a.Plan.Hotel.Slug != b.Plan.Hotel.Slug {
			t.Fatalf("same seed must give the same trip: %s vs %s", a.Plan.Hotel.Slug, b.Plan.Hotel.Slug)
		}
		if !a.Plan.Budget.Fits || a.Plan.Budget.Total > req.Budget {
			t.Fatalf("surprise must fit the budget: %d > %d", a.Plan.Budget.Total, req.Budget)
		}
		fit := 0
		for _, d := range a.Plan.Days {
			if d.WeatherFit != nil && *d.WeatherFit {
				fit++
			}
		}
		if fit*2 < len(a.Plan.Days) {
			t.Fatalf("surprise must mostly match the weather: %d of %d", fit, len(a.Plan.Days))
		}
		seen[a.Plan.Destination] = true
	}
	if len(seen) < 2 {
		t.Fatalf("15 seeds should not always give the same city: %v", seen)
	}
	for city := range seen {
		req.Exclude = append(req.Exclude, city)
	}
	for seed := int64(1); seed <= 5; seed++ {
		req.Seed = seed
		res, err := store.Surprise(ctx, 0, req)
		var empty *SurpriseEmptyError
		if errors.As(err, &empty) {
			break // все подходящие города исключены - честная ошибка тоже допустима
		}
		if err != nil || seen[res.Plan.Destination] {
			t.Fatalf("excluded city returned: %s err=%v", res.Plan.Destination, err)
		}
	}
	req = SurpriseRequest{TripRequest: TripRequest{From: from, To: addDays(from, 3), Guests: 2, Budget: 5000}}
	if _, err := store.Surprise(ctx, 0, req); err == nil || !strings.Contains(err.Error(), "бюджет") {
		t.Fatalf("impossible budget must explain what to relax, got %v", err)
	}
}
