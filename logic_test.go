package main

import "testing"

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
