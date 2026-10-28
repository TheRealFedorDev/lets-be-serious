package main

import (
	"fmt"
	"math"
	"sort"
)

// Паспорт совпадения: не просто «87% match», а разложение этого числа
// на понятные человеку причины. Сумма вкладов факторов = 100%.

type MatchFactor struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
	Points int     `json:"points"`
	Note   string  `json:"note,omitempty"`
	Tone   string  `json:"tone"`
}

type MatchBreakdown struct {
	Score   int           `json:"score"`
	Verdict string        `json:"verdict"`
	Factors []MatchFactor `json:"factors"`
	Pros    []string      `json:"pros"`
	Cons    []string      `json:"cons"`
}

type matchFeature struct {
	Key    string
	Label  string
	Note   string
	Value  float64
	Weight float64
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	return clampFloat(value, 0, 1)
}

func hotelMatchFeatures(hotel HotelCard, params SearchParams, moodSlug string, inferredTags []string) []matchFeature {
	features := make([]matchFeature, 0, 12)

	rating := clamp01(hotel.Rating / 10)
	features = append(features, matchFeature{
		Key: "rating", Label: "Рейтинг гостей", Note: fmt.Sprintf("%.1f из 10 · %d отзывов", hotel.Rating, hotel.Reviews),
		Value: rating, Weight: 2.4,
	})

	price := clamp01(1 - float64(hotel.PriceFrom)/40000.0)
	features = append(features, matchFeature{
		Key: "price", Label: "Цена за ночь", Note: fmt.Sprintf("%d ₽", hotel.PriceFrom),
		Value: price, Weight: 1.0,
	})

	tagOverlap := 0
	for _, tag := range inferredTags {
		if containsString(hotel.Meta.TagSlugs, tag) {
			tagOverlap++
		}
	}
	tagValue := 0.0
	if len(inferredTags) > 0 {
		tagValue = clamp01(float64(tagOverlap) / float64(len(inferredTags)))
	}
	features = append(features, matchFeature{
		Key: "tags", Label: "Совпадение по запросу", Note: fmt.Sprintf("%d из %d тегов", tagOverlap, len(inferredTags)),
		Value: tagValue, Weight: 2.8,
	})

	moodValue := 0.0
	if moodSlug != "" && containsString(hotel.Meta.TagSlugs, moodSlug) {
		moodValue = 1
	}
	if moodSlug != "" {
		features = append(features, matchFeature{
			Key: "mood", Label: "Настроение поездки", Note: moodTitle(moodSlug),
			Value: moodValue, Weight: 2.2,
		})
	}

	comfort := []struct {
		key, label string
		on         bool
		weight     float64
	}{
		{"insured", "Страховка проживания", hotel.Meta.Insured, 0.9},
		{"guides", "Услуги гидов", hotel.Meta.Guides, 0.8},
		{"pool", "Бассейн", hotel.Meta.Pool, 1.0},
		{"parking", "Парковка", hotel.Meta.Parking, 0.7},
		{"soundproof", "Тихие номера", hotel.Meta.Soundproof, 0.8},
	}
	for _, item := range comfort {
		value := 0.0
		if item.on {
			value = 1
		}
		features = append(features, matchFeature{
			Key: item.key, Label: item.label, Value: value, Weight: item.weight,
			Note: map[bool]string{true: "есть", false: "нет"}[item.on],
		})
	}

	if params.Lat != 0 || params.Lng != 0 {
		distance := hotel.DistanceKm
		if distance <= 0 {
			distance = 30
		}
		value := clamp01(1 - math.Min(distance, 50)/50.0)
		features = append(features, matchFeature{
			Key: "distance", Label: "Близость к вам", Note: fmt.Sprintf("%.1f км", distance),
			Value: value, Weight: 1.3,
		})
	}

	if params.MaxPrice > 0 {
		budgetValue := 1.0
		if hotel.PriceFrom > params.MaxPrice {
			budgetValue = clamp01(1 - float64(hotel.PriceFrom-params.MaxPrice)/float64(params.MaxPrice))
		}
		features = append(features, matchFeature{
			Key: "budget", Label: "Вписывается в бюджет",
			Note:  fmt.Sprintf("%d ₽ при лимите %d ₽", hotel.PriceFrom, params.MaxPrice),
			Value: budgetValue, Weight: 1.8,
		})
	}

	for _, check := range hotel.Needs {
		value := 0.55
		if check.Known {
			if check.OK {
				value = 1
			} else {
				value = 0
			}
		}
		note := check.Note
		if check.Kind == NeedKindNearby && check.Known {
			note = fmt.Sprintf("%d мин при лимите %d", check.Minutes, check.Limit)
		}
		features = append(features, matchFeature{
			Key: "need-" + check.Key, Label: check.Label, Note: note,
			Value: value, Weight: 1.6,
		})
	}
	return features
}

func logisticScore(features []matchFeature) int {
	const bias = -1.8
	z := bias
	for _, feature := range features {
		z += feature.Value * feature.Weight
	}
	probability := 1 / (1 + math.Exp(-z))
	return int(probability*100 + 0.5)
}

func logisticRecommendation(hotel HotelCard, params SearchParams, moodSlug string, inferredTags []string) int {
	return logisticScore(hotelMatchFeatures(hotel, params, moodSlug, inferredTags))
}

func BuildMatchBreakdown(hotel HotelCard, params SearchParams, moodSlug string, inferredTags []string) MatchBreakdown {
	features := hotelMatchFeatures(hotel, params, moodSlug, inferredTags)
	score := logisticScore(features)

	total := 0.0
	for _, feature := range features {
		total += feature.Value * feature.Weight
	}
	if total <= 0 {
		total = 1
	}

	factors := make([]MatchFactor, 0, len(features))
	sumPoints := 0
	for _, feature := range features {
		weighted := feature.Value * feature.Weight
		points := int(weighted/total*float64(score) + 0.5)
		sumPoints += points
		tone := "neutral"
		switch {
		case weighted <= 0.001:
			tone = "minus"
		case weighted/feature.Weight >= 0.66:
			tone = "plus"
		}
		factors = append(factors, MatchFactor{
			Key:    feature.Key,
			Label:  feature.Label,
			Note:   feature.Note,
			Value:  math.Round(feature.Value*100) / 100,
			Weight: feature.Weight,
			Points: points,
			Tone:   tone,
		})
	}
	// держим обещание «вклады складываются в балл»: гасим ошибку округления
	if len(factors) > 0 && sumPoints != score {
		delta := score - sumPoints
		best := 0
		for index := range factors {
			if factors[index].Points > factors[best].Points {
				best = index
			}
		}
		factors[best].Points += delta
	}
	sort.SliceStable(factors, func(i, j int) bool { return factors[i].Points > factors[j].Points })

	breakdown := MatchBreakdown{
		Score:   score,
		Verdict: tripVerdict(score),
		Factors: factors,
		Pros:    make([]string, 0, 3),
		Cons:    make([]string, 0, 2),
	}
	for _, factor := range factors {
		if factor.Tone == "plus" && len(breakdown.Pros) < 3 {
			breakdown.Pros = append(breakdown.Pros, fmt.Sprintf("%s — %s", factor.Label, factor.Note))
		}
	}
	for index := len(factors) - 1; index >= 0; index-- {
		factor := factors[index]
		if factor.Tone == "minus" && len(breakdown.Cons) < 2 {
			label := factor.Label
			if factor.Key == "price" {
				label = "Цена выше комфортной"
			}
			breakdown.Cons = append(breakdown.Cons, fmt.Sprintf("%s — %s", label, factor.Note))
		}
	}
	return breakdown
}

func moodTitle(slug string) string {
	if mood, ok := moodBySlug[slug]; ok {
		return mood.Title
	}
	return slug
}
