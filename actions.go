package main

import "strings"

// Concierge-действия: ассистент не только отвечает текстом, но и предлагает
// следующий шаг, который можно нажать. Действия детерминированные — они
// зависят от контекста (отель/город/сообщение), а не от фантазии модели.

type AssistantAction struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Kind      string   `json:"kind"` // trip | filter | prompt | hotel
	Icon      string   `json:"icon,omitempty"`
	Prompt    string   `json:"prompt,omitempty"`
	HotelSlug string   `json:"hotelSlug,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Primary   bool     `json:"primary,omitempty"`
	// Destination - slug города из контекста ответа: действие не теряет «Куда».
	Destination string `json:"destination,omitempty"`
	// Prefs - параметры, понятые из фразы: мастер поездки сразу собирает по ним вариант.
	Prefs *AssistantTripPrefs `json:"prefs,omitempty"`
}

func buildAssistantActions(destination Destination, focus *AssistantHotel, message string) []AssistantAction {
	actions := buildAssistantActionsRaw(destination, focus, message)
	prefs, understood := extractTripPrefs(message, destination)
	for index := range actions {
		if actions[index].Destination == "" && destination.Slug != "" {
			actions[index].Destination = destination.Slug
		}
		if understood && actions[index].Kind == "trip" {
			p := prefs
			actions[index].Prefs = &p
			if summary := tripPrefsSummary(prefs); summary != "" {
				actions[index].Label = "Собрать поездку: " + summary
			}
		}
	}
	if wantsSurprise(message) {
		p := prefs
		p.Destination = ""
		surprise := AssistantAction{ID: "surprise", Label: "🎲 Поездка-сюрприз", Kind: "surprise", Icon: "route", Primary: true, Prefs: &p}
		if summary := tripPrefsSummary(p); summary != "" {
			surprise.Label += ": " + summary
		}
		for index := range actions {
			actions[index].Primary = false
		}
		actions = append([]AssistantAction{surprise}, actions...)
	}
	return actions
}

// wantsSurprise - просьба выбрать поездку наугад.
func wantsSurprise(message string) bool {
	return containsAny(strings.ToLower(message), "удиви", "сюрприз", "рандом", "наугад", "мне повез", "куда угодно", "куда-нибудь")
}

func buildAssistantActionsRaw(destination Destination, focus *AssistantHotel, message string) []AssistantAction {
	actions := make([]AssistantAction, 0, 4)
	lower := strings.ToLower(message)

	if focus != nil {
		actions = append(actions, AssistantAction{
			ID: "trip", Label: "Собрать маршрут вокруг отеля", Kind: "trip", Icon: "route",
			HotelSlug: focus.Slug, Primary: true,
		})
		if !hasTag(focus.Tags, "zavtraki") && !strings.Contains(lower, "завтрак") {
			actions = append(actions, AssistantAction{
				ID: "breakfast", Label: "Только с завтраком", Kind: "filter", Icon: "coffee",
				Tags: []string{"zavtraki"},
			})
		}
		if strings.Contains(lower, "альтернатив") || strings.Contains(lower, "другой") || strings.Contains(lower, "похож") {
			actions = append(actions, AssistantAction{
				ID: "similar", Label: "Показать похожие отели", Kind: "prompt", Icon: "search",
				Prompt: "Покажи похожие отели в этом городе с ценой и рейтингом",
			})
		}
		if strings.Contains(" "+normalizeHotelName(message)+" ", " "+normalizeHotelName(focus.Name)+" ") {
			actions = append(actions, AssistantAction{
				ID: "open-hotel", Label: "Открыть отель", Kind: "hotel", Icon: "pin",
				HotelSlug: focus.Slug,
			})
		}
		actions = append(actions, AssistantAction{
			ID: "nearby", Label: "Что рядом с отелем", Kind: "prompt", Icon: "pin",
			Prompt: "Что интересного рядом с отелем и сколько идти пешком?",
		})
		if len(actions) < 3 {
			actions = append(actions, AssistantAction{
				ID: "price", Label: "Цена с налогами и завтраком", Kind: "prompt", Icon: "wallet",
				Prompt: "Сколько стоит номер с налогами, сборами и завтраком?",
			})
		}
		return actions[:min(4, len(actions))]
	}

	if destination.Slug == "" {
		// Город не распознан: предлагаем выбрать направление, а не гадать.
		for _, city := range []string{"Сочи", "Москва", "Казань", "Санкт-Петербург"} {
			actions = append(actions, AssistantAction{
				ID: "pick-" + city, Label: "Отели " + cityPhrase(city), Kind: "prompt", Icon: "pin",
				Prompt: "Подбери отели " + cityPhrase(city),
			})
		}
		actions[0].Primary = true
		return actions
	}

	actions = append(actions, AssistantAction{
		ID: "trip", Label: "Собрать маршрут поездки", Kind: "trip", Icon: "route",
		Primary: true,
	})
	if strings.Contains(lower, "завтрак") {
		actions = append(actions, AssistantAction{
			ID: "breakfast", Label: "Отели с завтраком", Kind: "filter", Icon: "coffee",
			Tags: []string{"zavtraki"},
		})
	} else {
		actions = append(actions, AssistantAction{
			ID: "breakfast", Label: "Только с завтраком", Kind: "filter", Icon: "coffee",
			Tags: []string{"zavtraki"},
		})
	}
	if strings.Contains(lower, "тих") || strings.Contains(lower, "шум") {
		actions = append(actions, AssistantAction{
			ID: "quiet", Label: "Тихие номера", Kind: "filter", Icon: "moon",
			Tags: []string{"tihie-nomera"},
		})
	} else if destination.Slug != "" {
		actions = append(actions, AssistantAction{
			ID: "city", Label: "Отели " + cityPhrase(destination.City), Kind: "filter", Icon: "pin",
		})
	}
	if len(actions) < 3 {
		actions = append(actions, AssistantAction{
			ID: "budget", Label: "Подобрать под бюджет", Kind: "prompt", Icon: "wallet",
			Prompt: "Подбери отель под мой бюджет — уточни, сколько готовы потратить за ночь",
		})
	}
	return actions[:min(4, len(actions))]
}

func hasTag(tags []string, tag string) bool {
	for _, item := range tags {
		if item == tag {
			return true
		}
	}
	return false
}
