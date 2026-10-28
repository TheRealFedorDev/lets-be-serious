package main

import (
	"net/http"
	"strings"
	"time"
)

// Demo mode: три готовых сценария для сцены.
// Нужны, чтобы демо не зависело от того, что зритель введёт в форму,
// и чтобы можно было показать продукт за 30 секунд.

type DemoScenario struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle"`
	Emoji    string      `json:"emoji"`
	Prefill  TripRequest `json:"prefill"`
	Plan     *TripPlan   `json:"plan,omitempty"`
}

func demoScenarios() []DemoScenario {
	start := time.Now().AddDate(0, 0, 21).Format("2006-01-02")
	return []DemoScenario{
		{
			ID: "family", Title: "Семья с ребёнком", Subtitle: "Сочи · море рядом, детская комната", Emoji: "🧸",
			Prefill: TripRequest{Destination: "sochi", From: start, To: addDays(start, 4), Guests: 3, Budget: 195000, Mood: "semya", Tags: []string{"plyazh", "semya"}, Pace: "calm"},
		},
		{
			ID: "romance", Title: "Романтика на двоих", Subtitle: "Санкт-Петербург · тишина и завтраки", Emoji: "🥂",
			Prefill: TripRequest{Destination: "saint-petersburg", From: start, To: addDays(start, 3), Guests: 2, Budget: 110000, Mood: "romantika", Tags: []string{"dlya-dvoih", "zavtraki"}, Pace: "calm"},
		},
		{
			ID: "budget", Title: "Город за разумные деньги", Subtitle: "Казань · максимум города без переплат", Emoji: "🎒",
			Prefill: TripRequest{Destination: "kazan", From: start, To: addDays(start, 3), Guests: 2, Budget: 75000, Mood: "na-pogulyat", Tags: []string{"budget", "na-pogulyat"}, Pace: "active"},
		},
	}
}

func demoScenarioByID(id string) (DemoScenario, bool) {
	for _, scenario := range demoScenarios() {
		if scenario.ID == id {
			return scenario, true
		}
	}
	return DemoScenario{}, false
}

func (api *API) handleDemoScenarios(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": demoScenarios()})
}

func (api *API) handleDemoScenario(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	scenario, ok := demoScenarioByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Сценарий не найден")
		return
	}
	plan, err := api.store.BuildTrip(r.Context(), api.optionalUserID(r), scenario.Prefill)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось собрать сценарий")
		return
	}
	scenario.Plan = &plan
	writeJSON(w, http.StatusOK, scenario)
}
