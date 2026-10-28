package main

import (
	"context"
	"fmt"
)

// What-if: «а что, если…» на карточке отеля.
// Считаем не на клиенте, а тем же движком, что и паспорт совпадения,
// чтобы цифры были честными и совпадали с основным баллом.

const (
	whatIfMinNights   = 1
	whatIfMaxNights   = 30
	whatIfMinGuests   = 1
	whatIfMaxGuests   = 10
	whatIfMinBudget   = 0
	whatIfMaxBudget   = 10_000_000
	whatIfBudgetPct   = 20 // «+20% к бюджету» — и когда бюджет задан, и когда нет
	whatIfFlexPremium = 8  // надбавка за свободную отмену, в процентах от цены за ночь
)

type WhatIfRequest struct {
	Slug   string   `json:"slug"`
	Guests int      `json:"guests"`
	Nights int      `json:"nights"`
	Budget int      `json:"budget"` // бюджет за ночь
	Mood   string   `json:"mood"`
	Tags   []string `json:"tags"`
}

// WhatIfInputError — вход вне допустимого диапазона.
// Обработчик должен отдавать его как 400 с Message (errors.As).
type WhatIfInputError struct {
	Message string
}

func (e *WhatIfInputError) Error() string { return e.Message }

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

func validateWhatIf(req WhatIfRequest) error {
	if req.Nights < whatIfMinNights || req.Nights > whatIfMaxNights {
		return &WhatIfInputError{Message: fmt.Sprintf("Количество ночей: от %d до %d", whatIfMinNights, whatIfMaxNights)}
	}
	if req.Guests < whatIfMinGuests || req.Guests > whatIfMaxGuests {
		return &WhatIfInputError{Message: fmt.Sprintf("Количество гостей: от %d до %d", whatIfMinGuests, whatIfMaxGuests)}
	}
	if req.Budget < whatIfMinBudget || req.Budget > whatIfMaxBudget {
		return &WhatIfInputError{Message: fmt.Sprintf("Бюджет за ночь: от %s до %s ₽", money(whatIfMinBudget), money(whatIfMaxBudget))}
	}
	return nil
}

// budgetLimit — лимит цены за ночь после «+20%»: от бюджета, а если он не задан — от цены отеля.
func budgetLimit(budget, basePrice int) int {
	anchor := budget
	if anchor <= 0 {
		anchor = basePrice
	}
	return anchor + anchor*whatIfBudgetPct/100
}

func (s *Store) WhatIf(ctx context.Context, userID int64, req WhatIfRequest) (WhatIfResponse, error) {
	if err := validateWhatIf(req); err != nil {
		return WhatIfResponse{}, err
	}
	guests := req.Guests
	nights := req.Nights

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

	// note — что меняется, если балл не изменился (цена или условия).
	build := func(key, label, description, note string, candidate HotelCard, candidateParams SearchParams, price int, candidateNights int) WhatIfVariant {
		breakdown := BuildMatchBreakdown(candidate, candidateParams, mood, tags)
		total := nightTotal(price, guests) * candidateNights
		variant := WhatIfVariant{
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
		if variant.Delta == 0 && note != "" {
			variant.Description = description + ". " + note
		}
		return variant
	}

	variants := make([]WhatIfVariant, 0, 4)

	budgetParams := params
	limit := budgetLimit(req.Budget, basePrice)
	budgetParams.MaxPrice = limit
	variants = append(variants, build(
		"budget",
		fmt.Sprintf("+%d%% к бюджету", whatIfBudgetPct),
		fmt.Sprintf("Лимит станет %s за ночь", money(limit)),
		"Балл не меняется: меняется только лимит цены, сам отель тот же",
		hotel, budgetParams, basePrice, nights,
	))

	// Завтрак уже входит в цену (buildPriceBreakdown всегда считает его строкой),
	// поэтому вместо «Добавить завтрак» показываем факт без доплаты.
	if breakfast := hotel.PriceBreakdown.Breakfast; breakfast > 0 {
		variants = append(variants, build(
			"breakfast",
			"Завтрак уже включён",
			fmt.Sprintf("Завтрак входит в цену: %s за ночь, доплата не нужна", money(breakfast)),
			"Цена и условия не меняются",
			hotel, params, basePrice, nights,
		))
	}

	extraHotel := hotel
	variants = append(variants, build(
		"nights",
		"+2 ночи",
		"Посмотреть стоимость длиннее на две ночи",
		"Балл не меняется: цена за ночь та же, растёт только итог за поездку",
		extraHotel, params, basePrice, nights+2,
	))

	flexHotel := hotel
	flexPrice := int(float64(basePrice) * (100 + whatIfFlexPremium) / 100)
	flexHotel.PriceFrom = flexPrice
	// Подпись — та же разница, что уходит в totalDelta (с налогами и сборами), иначе 4 ночи x подпись != итог.
	flexPerNight := nightTotal(flexPrice, guests) - nightTotal(basePrice, guests)
	variants = append(variants, build(
		"flexible",
		"Гибкая отмена",
		fmt.Sprintf("+%s за ночь за свободную отмену", money(flexPerNight)),
		"Балл не меняется: меняются условия отмены, а не отель",
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
