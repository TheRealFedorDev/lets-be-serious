package main

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type WalkStop struct {
	POIID       int64   `json:"poiId,omitempty"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Address     string  `json:"address,omitempty"`
	Description string  `json:"description,omitempty"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Dwell       int     `json:"dwell"`
	LegMeters   int     `json:"legMeters"`
	LegMinutes  int     `json:"legMinutes"`
	Schematic   bool    `json:"schematic,omitempty"`
}

type WalkingRoute struct {
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Indoor         bool       `json:"indoor"`
	Stops          []WalkStop `json:"stops"`
	DistanceMeters int        `json:"distanceMeters"`
	WalkMinutes    int        `json:"walkMinutes"`
	TotalMinutes   int        `json:"totalMinutes"`
	MapURL         string     `json:"mapUrl"`
	Schematic      bool       `json:"schematic,omitempty"`
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

func walkCandidates(pois []POI, rainy bool) []POI {
	out := make([]POI, 0)
	allowed := map[string]bool{"walk": true, "park": true, "nature": true, "attraction": true}
	if rainy {
		allowed = map[string]bool{"culture": true, "spa": true, "coffee": true, "food": true}
	} else {
		allowed["culture"] = true
	}
	for _, poi := range pois {
		if poi.Lat == 0 && poi.Lng == 0 {
			continue
		}
		if !allowed[poi.Category] {
			continue
		}
		out = append(out, poi)
	}
	return out
}

func planWalkingRoute(hotel HotelCard, pois []POI, pace string, rainy bool, seed int) *WalkingRoute {
	candidates := walkCandidates(pois, rainy)
	if len(candidates) < 2 {
		return nil
	}
	// rotate candidates for variety across days
	if seed > 0 && len(candidates) > 3 {
		rot := seed % len(candidates)
		rotated := append([]POI{}, candidates[rot:]...)
		rotated = append(rotated, candidates[:rot]...)
		candidates = rotated
	}
	count := 3
	if pace == "active" {
		count = 5
	}
	if count > len(candidates) {
		count = len(candidates)
	}
	// pick nearest-to-hotel first
	type scored struct {
		poi  POI
		dist float64
	}
	scoredList := make([]scored, 0, len(candidates))
	for _, poi := range candidates {
		scoredList = append(scoredList, scored{poi: poi, dist: haversineMeters(hotel.Meta.Lat, hotel.Meta.Lng, poi.Lat, poi.Lng)})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].dist < scoredList[j].dist })
	// prefer walkable cluster: keep only POIs within 3km; expand to 5km if needed
	if len(scoredList) > count {
		for _, radius := range []float64{3000, 5000} {
			near := make([]scored, 0)
			for _, sc := range scoredList {
				if sc.dist <= radius {
					near = append(near, sc)
				}
			}
			if len(near) >= count {
				scoredList = near
				break
			}
		}
	}
	pickCount := count * 2
	if pickCount > len(scoredList) {
		pickCount = len(scoredList)
	}
	pool := scoredList[:pickCount]

	// nearest neighbor
	remaining := make([]scored, len(pool))
	copy(remaining, pool)
	stops := make([]POI, 0, count)
	curLat, curLng := hotel.Meta.Lat, hotel.Meta.Lng
	if curLat == 0 && curLng == 0 && len(pool) > 0 {
		curLat, curLng = pool[0].poi.Lat, pool[0].poi.Lng
	}
	for len(stops) < count && len(remaining) > 0 {
		best := 0
		bestDist := math.Inf(1)
		for i, r := range remaining {
			d := haversineMeters(curLat, curLng, r.poi.Lat, r.poi.Lng)
			if d < bestDist {
				bestDist = d
				best = i
			}
		}
		stops = append(stops, remaining[best].poi)
		curLat, curLng = remaining[best].poi.Lat, remaining[best].poi.Lng
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	// 2-opt improvement
	improve2Opt(stops)
	// build legs
	routeStops := make([]WalkStop, 0, len(stops))
	totalMeters := 0
	walkMinutes := 0
	curLat, curLng = hotel.Meta.Lat, hotel.Meta.Lng
	if curLat == 0 && curLng == 0 && len(stops) > 0 {
		curLat, curLng = stops[0].Lat, stops[0].Lng
	}
	for _, poi := range stops {
		leg := haversineMeters(curLat, curLng, poi.Lat, poi.Lng) * 2.0
		legM := int(leg)
		legMin := int(leg/1000.0/4.5*60.0) + 1
		if legMin < 1 {
			legMin = 1
		}
		dwell := poi.Minutes
		if dwell < 25 {
			dwell = 25
		}
		if dwell > 60 {
			dwell = 60
		}
		routeStops = append(routeStops, WalkStop{
			POIID:       poi.ID,
			Name:        poi.Name,
			Category:    poi.Category,
			Address:     poi.Address,
			Description: poi.Description,
			Lat:         poi.Lat,
			Lng:         poi.Lng,
			Dwell:       dwell,
			LegMeters:   legM,
			LegMinutes:  legMin,
			Schematic:   poi.Schematic,
		})
		totalMeters += legM
		walkMinutes += legMin
		curLat, curLng = poi.Lat, poi.Lng
	}
	totalMinutes := walkMinutes
	for _, s := range routeStops {
		totalMinutes += s.Dwell
	}
	indoor := rainy
	title := "Пешая прогулка по району"
	if rainy {
		title = "Прогулка под крышей"
	}
	if pace == "active" {
		title = "Активный маршрут пешком"
	}
	summary := fmt.Sprintf("%d остановок · %d м пешком · около %d минут", len(routeStops), totalMeters, totalMinutes)
	// yandex link
	coords := []string{}
	if hotel.Meta.Lat != 0 || hotel.Meta.Lng != 0 {
		coords = append(coords, fmt.Sprintf("%.6f,%.6f", hotel.Meta.Lat, hotel.Meta.Lng))
	}
	for _, s := range routeStops {
		coords = append(coords, fmt.Sprintf("%.6f,%.6f", s.Lat, s.Lng))
	}
	mapURL := ""
	if len(coords) >= 2 {
		mapURL = "https://yandex.ru/maps/?rtext=" + joinStrings(coords, "~") + "&rtt=pd"
	}
	schematic := false
	for _, s := range routeStops {
		if s.Schematic {
			schematic = true
		}
	}
	if hotel.Meta.Lat == 0 && hotel.Meta.Lng == 0 {
		schematic = true
	}
	return &WalkingRoute{
		Title:          title,
		Summary:        summary,
		Indoor:         indoor,
		Stops:          routeStops,
		DistanceMeters: totalMeters,
		WalkMinutes:    walkMinutes,
		TotalMinutes:   totalMinutes,
		MapURL:         mapURL,
		Schematic:      schematic,
	}
}

func improve2Opt(stops []POI) {
	if len(stops) < 4 {
		return
	}
	dist := func(a, b POI) float64 { return haversineMeters(a.Lat, a.Lng, b.Lat, b.Lng) }
	improved := true
	for improved {
		improved = false
		for i := 1; i < len(stops)-1; i++ {
			for j := i + 1; j < len(stops); j++ {
				// 2-opt: reverse segment [i..j]
				prev := stops[i-1]
				var next POI
				if j+1 < len(stops) {
					next = stops[j+1]
				} else {
					next = stops[0]
				}
				cur := dist(prev, stops[i]) + dist(stops[j], next)
				newd := dist(prev, stops[j]) + dist(stops[i], next)
				if newd+0.01 < cur {
					// reverse segment
					for a, b := i, j; a < b; a, b = a+1, b-1 {
						stops[a], stops[b] = stops[b], stops[a]
					}
					improved = true
				}
			}
		}
	}
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// BuildWalk is a helper for handlers/hotel detail.
func (s *Store) BuildWalk(ctx context.Context, hotel HotelCard, pois []POI, pace string, rainy bool) *WalkingRoute {
	return planWalkingRoute(hotel, pois, pace, rainy, 0)
}
