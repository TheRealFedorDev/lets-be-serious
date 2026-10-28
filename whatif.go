package main

import (
	"context"
	"fmt"
)

// What-if: «а что, если…» на карточке отеля.
// Считаем не на клиенте, а тем же движком, что и паспорт совпадения,
// чтобы цифры были честными и совпадали с основным баллом.

type WhatIfRequest struct {
	Slug   string   `json:"slug"`
	Guests int      `json:"guests"`
	Nights int      `json:"nights"`
	Budget int      `json:"budget"` // бюджет за ночь
	Mood   string   `json:"mood"`
	Tags   []string `json:"tags"`
}

type WhatIfVariant struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	Delta       int    `json:"delta"`
	Price       int    `json:"price"`
	PriceDelta  int    `json:"priceDelta"`
	Total       int    `json:"total"`
	TotalDelta  int    `json:"totalDelta"`
}

type WhatIfResponse struct {
	Score    int             `json:"score"`
	Price    int             `json:"price"`
	Total    int             `json:"total"`
	Variants []WhatIfVariant `json:"variants"`
}

func (s *Store) WhatIf(ctx context.Context, userID int64, req WhatIfRequest) (WhatIfResponse, error) {
	guests := req.Guests
	if guests <= 0 {
		guests = 2
	}
	nights := req.Nights
	if nights <= 0 {
		nights = 1
	}

	detail, err := s.HotelBySlug(ctx, userID, req.Slug, "", "", guests)
	if err != nil {
		return WhatIfResponse{}, err
	}
	hotel := detail.HotelCard
	mood := req.Mood
	tags := NormalizeTags(req.Tags)

	params := SearchParams{Guests: guests, Mood: mood, Tags: tags, MaxPrice: req.Budget}
	base := BuildMatchBreakdown(hotel, params, mood, tags)
	basePrice := hotel.PriceFrom
	baseTotal := nightTotal(basePrice, guests) * nights

	build := func(key, label, description string, candidate HotelCard, candidateParams SearchParams, price int, candidateNights int) WhatIfVariant {
		breakdown := BuildMatchBreakdown(candidate, candidateParams, mood, tags)
		total := nightTotal(price, guests) * candidateNights
		return WhatIfVariant{
			Key:         key,
			Label:       label,
			Description: description,
			Score:       breakdown.Score,
			Delta:       breakdown.Score - base.Score,
			Price:       price,
			PriceDelta:  price - basePrice,
			Total:       total,
			TotalDelta:  total - baseTotal,
		}
	}

	variants := make([]WhatIfVariant, 0, 4)

	budgetParams := params
	raise := 0
	if req.Budget > 0 {
		raise = int(float64(req.Budget) * 0.2)
		budgetParams.MaxPrice = req.Budget + raise
	} else {
		raise = int(float64(basePrice) * 0.25)
		budgetParams.MaxPrice = basePrice + raise
	}
	variants = append(variants, build(
		"budget",
		"+20% к бюджету",
		fmt.Sprintf("Лимит станет %s за ночь", money(budgetParams.MaxPrice)),
		hotel, budgetParams, basePrice, nights,
	))

	breakfastHotel := hotel
	breakfastPrice := basePrice + 500*guests
	breakfastHotel.PriceFrom = breakfastPrice
	variants = append(variants, build(
		"breakfast",
		"Добавить завтрак",
		fmt.Sprintf("+%s за ночь на всех гостей", money(500*guests)),
		breakfastHotel, params, breakfastPrice, nights,
	))

	extraHotel := hotel
	variants = append(variants, build(
		"nights",
		"+2 ночи",
		"Посмотреть стоимость длиннее на две ночи",
		extraHotel, params, basePrice, nights+2,
	))

	flexHotel := hotel
	flexPrice := int(float64(basePrice) * 1.08)
	flexHotel.PriceFrom = flexPrice
	variants = append(variants, build(
		"flexible",
		"Гибкая отмена",
		fmt.Sprintf("+%s за ночь за свободную отмену", money(flexPrice-basePrice)),
		flexHotel, params, flexPrice, nights,
	))

	return WhatIfResponse{
		Score:    base.Score,
		Price:    basePrice,
		Total:    baseTotal,
		Variants: variants,
	}, nil
}

func nightTotal(price, guests int) int {
	breakdown := buildPriceBreakdown(price, guests)
	if breakdown.Total > 0 {
		return breakdown.Total
	}
	return price
}
