package geppetto

import "testing"

func BenchmarkScoreAction(b *testing.B) {
	agent := Agent{Considerations: map[string]Consideration{
		"HUNGER": {ID: "HUNGER", Value: -70, Min: -100, Max: 100, BaseWeight: 3, ResponseCurve: ResponseCurve{Kind: Convex, Exponent: 2}},
		"ENERGY": {ID: "ENERGY", Value: -20, Min: -100, Max: 100, BaseWeight: 2, ResponseCurve: ResponseCurve{Kind: Convex, Exponent: 2}},
	}, Resources: map[string]float64{"ammo": 10}}
	provider := AffordanceProvider{Position: Position{X: 5}}
	action := AdvertisedAction{ActionID: "eat", Deltas: map[string]float64{"HUNGER": 80, "ENERGY": -5}, AdvertisementRadius: 10}
	tuning := DefaultTuning()
	b.ReportAllocs()
	for b.Loop() {
		_ = ScoreAction(agent, provider, action, tuning)
	}
}
