package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// «Поездка-сюрприз»: пользователь задаёт бюджет, даты и пожелания, а город и отель выпадают случайно -
// но только среди вариантов, которые проходят все условия. Выбор взвешенный: лучше подходит - выше шанс.
// Генератор с явным seed, поэтому результат воспроизводим (ссылка, повторная сборка при оплате).

type SurpriseRequest struct {
	TripRequest
	Exclude []string `json:"exclude"` // slug городов, которые не предлагать
	Seed    int64    `json:"seed"`
}

type SurpriseResult struct {
	Plan       TripPlan `json:"plan"`
	Seed       int64    `json:"seed"`
	Candidates int      `json:"candidates"`
	Why        []string `json:"why"`
	Cities     []string `json:"cities"` // все подходящие города - для анимации «барабана»
}

// SurpriseEmptyError - под условия ничего нет; Hint подсказывает, что ослабить.
type SurpriseEmptyError struct{ Hint string }

func (e *SurpriseEmptyError) Error() string { return e.Hint }

type surpriseCandidate struct {
	plan   TripPlan
	weight int
	why    []string
}

const maxSurpriseExclude = 20

func (s *Store) surpriseCandidates(ctx context.Context, userID int64, req SurpriseRequest, checkBudget, checkWeather bool) ([]surpriseCandidate, error) {
	destinations, err := s.Destinations(ctx)
	if err != nil {
		return nil, err
	}
	excluded := map[string]bool{}
	for index, slug := range req.Exclude {
		if index >= maxSurpriseExclude {
			break
		}
		excluded[strings.TrimSpace(slug)] = true
	}
	out := make([]surpriseCandidate, 0, len(destinations))
	for _, destination := range destinations {
		if excluded[destination.Slug] {
			continue
		}
		trip := req.TripRequest
		trip.Destination = destination.Slug
		trip.HotelSlug = ""
		plan, err := s.BuildTrip(ctx, userID, trip)
		if err != nil {
			continue // нет отелей под обязательные условия - город не участвует
		}
		if plan.Room == nil {
			continue
		}
		why := []string{}
		if checkBudget && req.Budget > 0 {
			if !plan.Budget.Fits {
				continue
			}
			why = append(why, fmt.Sprintf("укладывается в бюджет: %s из %s", money(plan.Budget.Total), money(req.Budget)))
		}
		fit := 0
		for _, day := range plan.Days {
			if day.WeatherFit == nil || *day.WeatherFit {
				fit++
			}
		}
		if checkWeather && req.Weather != "" {
			if fit*2 < len(plan.Days) {
				continue
			}
			why = append(why, fmt.Sprintf("погода подходит %d из %d дней", fit, len(plan.Days)))
		}
		interest := 0
		for _, place := range plan.Nearby {
			if place.Match {
				interest++
			}
		}
		if interest > 0 {
			why = append(why, fmt.Sprintf("рядом с отелем %d %s под ваши интересы", interest, pluralWord(interest, "место", "места", "мест")))
		}
		why = append(why, fmt.Sprintf("совпадение с отелем %d%%", plan.Match.Score))
		score := plan.Match.Score + fit*5 + interest*4
		out = append(out, surpriseCandidate{plan: plan, weight: score * score, why: why})
	}
	return out, nil
}

// Surprise выбирает поездку случайно среди подходящих. Повтор того же seed даёт тот же результат.
func (s *Store) Surprise(ctx context.Context, userID int64, req SurpriseRequest) (SurpriseResult, error) {
	if req.Seed <= 0 {
		req.Seed = time.Now().UnixNano()%1_000_000_000 + 1
	}
	candidates, err := s.surpriseCandidates(ctx, userID, req, true, true)
	if err != nil {
		return SurpriseResult{}, err
	}
	if len(candidates) == 0 {
		return SurpriseResult{}, &SurpriseEmptyError{Hint: s.surpriseHint(ctx, userID, req)}
	}
	total := 0
	for _, c := range candidates {
		total += c.weight
	}
	rng := rand.New(rand.NewSource(req.Seed))
	roll := rng.Intn(total)
	chosen := candidates[len(candidates)-1]
	for _, c := range candidates {
		if roll < c.weight {
			chosen = c
			break
		}
		roll -= c.weight
	}
	cities := make([]string, 0, len(candidates))
	for _, c := range candidates {
		cities = append(cities, c.plan.City)
	}
	return SurpriseResult{Plan: chosen.plan, Seed: req.Seed, Candidates: len(candidates), Why: chosen.why, Cities: cities}, nil
}

// surpriseHint подсказывает, какое одно условие ослабить, чтобы варианты появились.
func (s *Store) surpriseHint(ctx context.Context, userID int64, req SurpriseRequest) string {
	if req.Budget > 0 {
		if c, err := s.surpriseCandidates(ctx, userID, req, false, true); err == nil && len(c) > 0 {
			return "Под этот бюджет ничего не нашлось — увеличьте его или сократите поездку."
		}
	}
	if req.Weather != "" {
		if c, err := s.surpriseCandidates(ctx, userID, req, true, false); err == nil && len(c) > 0 {
			return "На эти даты нигде нет нужной погоды — смените даты или выберите «не важно»."
		}
	}
	if req.Needs != "" {
		return "Обязательные условия отсекли все отели — переведите одно из них в «желательно»."
	}
	if len(req.Exclude) > 0 {
		return "Остальные города исключены — уберите исключения."
	}
	return "Под эти условия ничего нет — ослабьте одно из них."
}
