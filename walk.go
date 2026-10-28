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

const (
	walkDetourFactor = 1.4  // прямая -> реальная длина пути по улицам
	walkSpeedKmh     = 4.5  // скорость ходьбы
	walkMaxTotalM    = 8000 // предел суммарной дистанции маршрута
	walkMaxMinutes   = 150  // предел суммарного времени ходьбы
	walkMaxLegM      = 3000 // предел одного перехода
	walkMinStops     = 2    // меньше двух остановок - маршрута нет
)

// openAirCulture - точки категории culture, которые на самом деле под открытым небом
// или активности: в дождь в «прогулку под крышей» они не попадают.
var openAirCulture = map[string]bool{
	"Скайпарк":                 true,
	"Морской вокзал":           true,
	"Дворец земледельцев":      true,
	"Петропавловская крепость": true,
	"Новодевичий монастырь":    true,
}

var openAirTags = map[string]bool{"активность": true, "горы": true, "море": true, "прогулка": true}

// isIndoorPOI решает, можно ли считать точку «под крышей».
func isIndoorPOI(poi POI) bool {
	switch poi.Category {
	case "spa", "coffee", "food":
		return true
	case "culture":
		if openAirCulture[poi.Name] {
			return false
		}
		for _, t := range poi.Tags {
			if openAirTags[t] {
				return false
			}
		}
		return true
	}
	return false
}

func walkCandidates(pois []POI, rainy bool) []POI {
	out := make([]POI, 0)
	allowed := map[string]bool{"walk": true, "park": true, "nature": true, "attraction": true, "culture": true}
	for _, poi := range pois {
		if poi.Lat == 0 && poi.Lng == 0 {
			continue
		}
		if rainy {
			if !isIndoorPOI(poi) {
				continue
			}
		} else if !allowed[poi.Category] {
			continue
		}
		out = append(out, poi)
	}
	return out
}

func planWalkingRoute(hotel HotelCard, pois []POI, pace string, rainy bool, seed int) *WalkingRoute {
	candidates := walkCandidates(pois, rainy)
	if len(candidates) < walkMinStops {
		return nil
	}
	count := 3
	if pace == "active" {
		count = 5
	}
	startLat, startLng := hotel.Meta.Lat, hotel.Meta.Lng
	noHotel := startLat == 0 && startLng == 0
	if noHotel {
		startLat, startLng = candidates[0].Lat, candidates[0].Lng
	}
	// оставляем только то, что реально достижимо пешком от отеля
	maxStraight := float64(walkMaxLegM) / walkDetourFactor
	type scored struct {
		poi  POI
		dist float64
	}
	near := make([]scored, 0, len(candidates))
	for _, poi := range candidates {
		d := haversineMeters(startLat, startLng, poi.Lat, poi.Lng)
		if d <= maxStraight {
			near = append(near, scored{poi: poi, dist: d})
		}
	}
	if len(near) < walkMinStops {
		return nil
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].dist < near[j].dist })
	pickCount := count * 2
	if pickCount > len(near) {
		pickCount = len(near)
	}
	// окно по ближайшим, сдвигается по дням для разнообразия
	offset := 0
	if seed > 0 && len(near) > count {
		offset = (seed * count) % len(near)
	}
	pool := make([]scored, 0, pickCount)
	for i := 0; i < pickCount; i++ {
		pool = append(pool, near[(offset+i)%len(near)])
	}

	// nearest neighbor от отеля
	stops := make([]POI, 0, count)
	curLat, curLng := startLat, startLng
	for len(stops) < count && len(pool) > 0 {
		best := 0
		bestDist := math.Inf(1)
		for i, r := range pool {
			d := haversineMeters(curLat, curLng, r.poi.Lat, r.poi.Lng)
			if d < bestDist {
				bestDist = d
				best = i
			}
		}
		stops = append(stops, pool[best].poi)
		curLat, curLng = pool[best].poi.Lat, pool[best].poi.Lng
		pool = append(pool[:best], pool[best+1:]...)
	}
	improve2Opt(startLat, startLng, stops)
	// build legs
	routeStops := make([]WalkStop, 0, len(stops))
	totalMeters := 0
	walkMinutes := 0
	curLat, curLng = hotel.Meta.Lat, hotel.Meta.Lng
	if curLat == 0 && curLng == 0 && len(stops) > 0 {
		curLat, curLng = stops[0].Lat, stops[0].Lng
	}
	for _, poi := range stops {
		leg := haversineMeters(curLat, curLng, poi.Lat, poi.Lng) * walkDetourFactor
		legM := int(leg)
		legMin := int(leg/1000.0/walkSpeedKmh*60.0) + 1
		if legM > walkMaxLegM || totalMeters+legM > walkMaxTotalM || walkMinutes+legMin > walkMaxMinutes {
			break
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
	if len(routeStops) < walkMinStops {
		return nil
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

// improve2Opt улучшает открытый путь (старт в точке startLat/startLng, возврата нет).
func improve2Opt(startLat, startLng float64, stops []POI) {
	if len(stops) < 3 {
		return
	}
	at := func(i int) (float64, float64) {
		if i < 0 {
			return startLat, startLng
		}
		return stops[i].Lat, stops[i].Lng
	}
	d := func(i, j int) float64 {
		la, lo := at(i)
		lb, lob := at(j)
		return haversineMeters(la, lo, lb, lob)
	}
	improved := true
	for improved {
		improved = false
		for i := 0; i < len(stops)-1; i++ {
			for j := i + 1; j < len(stops); j++ {
				cur := d(i-1, i)
				nw := d(i-1, j)
				if j+1 < len(stops) {
					cur += d(j, j+1)
					nw += d(i, j+1)
				}
				if nw+0.01 < cur {
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
