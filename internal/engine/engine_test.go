package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAgent(cs ...Consideration) Agent {
	return Agent{ID: "agent", Considerations: considerations(cs), Position: Position{}, Resources: map[string]float64{"ammo": 100}}
}

func considerations(cs []Consideration) map[string]Consideration {
	result := make(map[string]Consideration, len(cs))
	for _, c := range cs {
		result[c.ID] = c
	}
	return result
}

func c(id string, value float64) Consideration {
	return Consideration{ID: id, Value: value, Min: -100, Max: 100, BaseWeight: 1, CriticalThreshold: -50, ResponseCurve: ResponseCurve{Kind: Convex, Exponent: 2}}
}

func action(id string, deltas map[string]float64, tags ...string) AdvertisedAction {
	return AdvertisedAction{ActionID: id, Deltas: deltas, EstimatedDuration: 1, Tags: tags, AdvertisementRadius: 100}
}

func provider(id string, p Position, actions ...AdvertisedAction) AffordanceProvider {
	return AffordanceProvider{ID: id, Position: p, Capacity: 1, AdvertisedActions: actions}
}

// With no action executed, a linear_decay consideration with decay_rate 10 pts/h
// goes from +100 to -100 in ~20 h of game time (±10%). Event-driven
// considerations do not move without events; perception-driven considerations
// respond to environment changes within 1 tick.
func TestConsiderationUpdatersDriveValues(t *testing.T) {
	decay := c("HUNGER", 100)
	decay.Updater = ConsiderationUpdater{Kind: LinearDecay, Rate: 10}
	event := c("AMMO", 50)
	event.Updater = ConsiderationUpdater{Kind: EventDriven}
	perception := c("THREAT", 0)
	perception.Updater = ConsiderationUpdater{Kind: PerceptionDriven, Value: func(Agent, World) float64 { return -75 }}
	a := testAgent(decay, event, perception)
	UpdateConsiderations(&a, World{}, 20)
	assert.InDelta(t, -100, a.Considerations["HUNGER"].Value, 10)
	assert.InDelta(t, 50, a.Considerations["AMMO"].Value, 0.0001)
	assert.InDelta(t, -75, a.Considerations["THREAT"].Value, 0.0001)
}

func TestRelationshipDrivenUpdaterUsesRelationshipValueWhenNoFunctionIsConfigured(t *testing.T) {
	bond := c("BOND:player", 0)
	bond.Updater = ConsiderationUpdater{Kind: RelationshipDriven}
	agent := testAgent(bond)

	UpdateConsiderations(&agent, World{Relationships: map[string]float64{"BOND:player": 40}}, 1)

	assert.InDelta(t, 40, agent.Considerations["BOND:player"].Value, 0.0001)
}

func TestContextAggregateUpdaterUsesContextValueWhenNoFunctionIsConfigured(t *testing.T) {
	morale := c("GROUP_MORALE", 0)
	morale.Updater = ConsiderationUpdater{Kind: ContextAggregate}
	agent := testAgent(morale)

	UpdateConsiderations(&agent, World{ContextValues: map[string]float64{"GROUP_MORALE": -30}}, 1)

	assert.InDelta(t, -30, agent.Considerations["GROUP_MORALE"].Value, 0.0001)
}

// Instantiating a new provider advertising {rest: ENERGY +80} in an environment
// with no previous ENERGY source makes agents with ENERGY below the critical
// threshold use it, with no change to agent code.
func TestAffordanceExtensibility(t *testing.T) {
	a := testAgent(c("ENERGY", -80))
	chosen := SelectAction(a, []AffordanceProvider{provider("bed", Position{}, action("rest", map[string]float64{"ENERGY": 80}))}, DefaultTuning(), rand.New(rand.NewPCG(1, 2)))
	require.NotNil(t, chosen)
	assert.Equal(t, "rest", chosen.Action.ActionID)
}

// An agent with HUNGER = -70 and FUN = -10, facing equidistant food and
// leisure providers with equivalent deltas, chooses to eat in at least 90% of
// 200 samples (with default SELECTION_TEMPERATURE).
func TestUrgencyDominates(t *testing.T) {
	a := testAgent(c("HUNGER", -70), c("FUN", -10))
	ps := []AffordanceProvider{provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80})), provider("fun", Position{}, action("play", map[string]float64{"FUN": 80}))}
	eat := 0
	for i := uint64(0); i < 200; i++ {
		if SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(i, i+7))).Action.ActionID == "eat" {
			eat++
		}
	}
	assert.GreaterOrEqual(t, eat, 180)
}

// An agent with ENERGY = +95 never chooses rest when any other action with
// utility > 0 is available: saturation caps the expected gain.
func TestSaturationCapsExpectedGain(t *testing.T) {
	a := testAgent(c("ENERGY", 95), c("FUN", -80))
	ps := []AffordanceProvider{provider("bed", Position{}, action("rest", map[string]float64{"ENERGY": 80})), provider("game", Position{}, action("play", map[string]float64{"FUN": 80}))}
	for i := uint64(0); i < 50; i++ {
		assert.Equal(t, "play", SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(i, 9))).Action.ActionID)
	}
}

// Two identical agents differing only in preference(book) = +8 vs
// preference(arena) = +8, with FUN = -50 and both providers available, diverge
// in their choice in at least 80% of the samples.
func TestPersonalityChangesChoices(t *testing.T) {
	base := testAgent(c("FUN", -50))
	book := base
	book.Personality.Preferences = map[string]float64{"book": 8}
	arena := base
	arena.Personality.Preferences = map[string]float64{"arena": 8}
	ps := []AffordanceProvider{provider("library", Position{}, action("read", map[string]float64{"FUN": 40})), provider("arena", Position{}, action("fight", map[string]float64{"FUN": 40}))}
	ps[0].AdvertisedActions[0].Domain = "book"
	ps[1].AdvertisedActions[0].Domain = "arena"
	different := 0
	for i := uint64(0); i < 100; i++ {
		if SelectAction(book, ps, DefaultTuning(), rand.New(rand.NewPCG(i, 3))).Action.ActionID != SelectAction(arena, ps, DefaultTuning(), rand.New(rand.NewPCG(i, 3))).Action.ActionID {
			different++
		}
	}
	assert.GreaterOrEqual(t, different, 80)
}

// In 1,000 decisions with a standout candidate (2x the runner-up's utility),
// the best option is chosen 60-90% of the time — never 100%
// (SELECTION_TEMPERATURE > 0 is guaranteed) and never at or below
// 1/SELECTION_TOP_K (it is not pure randomness).
func TestStochasticSelectionIsImperfect(t *testing.T) {
	a := testAgent(Consideration{ID: "X", Value: 0, Min: -100, Max: 100, BaseWeight: 1, ResponseCurve: ResponseCurve{Kind: Linear}})
	ps := []AffordanceProvider{provider("a", Position{}, action("best", map[string]float64{"X": 4})), provider("b", Position{}, action("second", map[string]float64{"X": 2}))}
	best := 0
	for i := uint64(0); i < 1000; i++ {
		if SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(i, i+1))).Action.ActionID == "best" {
			best++
		}
	}
	assert.GreaterOrEqual(t, best, 600)
	assert.LessOrEqual(t, best, 900)
}

// Doubling a provider's distance reduces its utility by the factor predicted
// by the distance multiplier M_distance, with all other terms held constant.
func TestDistanceMultiplier(t *testing.T) {
	a := testAgent(c("HUNGER", -50))
	x := action("eat", map[string]float64{"HUNGER": 20})
	near := provider("near", Position{X: 10}, x)
	far := provider("far", Position{X: 20}, x)
	n := ScoreAction(a, near, x, DefaultTuning())
	f := ScoreAction(a, far, x, DefaultTuning())
	assert.InDelta(t, (1.0/(1+2))/(1.0/(1+1)), f/n, 0.0001)
}

// An action whose preconditions (skill, faction, item, state) are not
// satisfied never appears among the scored candidates; upon gaining the
// skill/faction, the advertisement becomes eligible without restarting the
// agent.
func TestPreconditionsAreDynamic(t *testing.T) {
	a := testAgent(c("FUN", -80))
	locked := action("study", map[string]float64{"FUN": 80})
	locked.Preconditions = []Precondition{{Capability: "reading"}}
	assert.Empty(t, Candidates(a, []AffordanceProvider{provider("book", Position{}, locked)}, DefaultTuning()))
	a.Capabilities = map[string]bool{"reading": true}
	assert.Len(t, Candidates(a, []AffordanceProvider{provider("book", Position{}, locked)}, DefaultTuning()), 1)
}

// Preconditions on the provider's state also gate eligibility: an unloaded
// weapon does not advertise shoot.
func TestCandidatesRejectActionWhenProviderStateDoesNotMeetRequiredState(t *testing.T) {
	agent := testAgent(c("FUN", -80))
	advertisedAction := action("shoot", map[string]float64{"FUN": 80})
	advertisedAction.Preconditions = []Precondition{{RequiredState: "loaded"}}
	provider := provider("weapon", Position{}, advertisedAction)
	provider.State = "empty"

	assert.Empty(t, Candidates(agent, []AffordanceProvider{provider}, DefaultTuning()))
}

// An agent entering a location with a Context x2.0 modifier on the exercise
// tag increases the frequency of actions with that tag by at least 2x during
// the stay, vs. the baseline outside it.
func TestContextMultipliesExerciseFrequency(t *testing.T) {
	base := testAgent(c("FUN", -50))
	contextual := base
	contextual.ActiveContexts = []Context{{Modifiers: map[string]float64{"exercise": 2}}}
	ps := []AffordanceProvider{provider("gym", Position{}, action("exercise", map[string]float64{"FUN": 4}, "exercise")), provider("chair", Position{}, action("sit", map[string]float64{"FUN": 4}, "leisure"))}
	count := func(a Agent) int {
		n := 0
		for i := uint64(0); i < 1000; i++ {
			if SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(i, 20))).Action.ActionID == "exercise" {
				n++
			}
		}
		return n
	}
	assert.GreaterOrEqual(t, count(contextual), 2*count(base))
}

// An agent executing a leisure action with a consideration crossing the
// critical_threshold interrupts the action and seeks a provider that addresses
// it within 1 decision tick. Interruption requires the best available
// candidate's utility to strictly exceed preemption_margin x the current
// action's utility — tying the margin does not preempt. Without a critical
// consideration, the current action is never interrupted.
func TestPreemptionAboveMargin(t *testing.T) {
	a := testAgent(c("HUNGER", -60))
	a.CurrentAction = &ActionInstance{Action: action("leisure", map[string]float64{"HUNGER": 1}), ContinuationUtility: 1}
	ps := []AffordanceProvider{provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))}
	assert.True(t, CheckPreemption(&a, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 1))))
	assert.Equal(t, "eat", a.CurrentAction.Action.ActionID)
	// Equal to the margin is deliberately not enough: the spec requires strictly above.
	a.CurrentAction = &ActionInstance{Action: action("leisure", nil), ContinuationUtility: ScoreAction(a, ps[0], ps[0].AdvertisedActions[0], DefaultTuning()) / DefaultTuning().PreemptionMargin}
	assert.False(t, CheckPreemption(&a, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 1))))
}

// An agent in SIMPLIFIED mode whose aggregated schedule included a "meal" at
// 12h and returns to FULL mode at 13h reappears with HUNGER >= +30 — a
// plausible state, not starving. The reconstruction table is declared as data,
// the same way a profile would declare it.
func TestLODReconstructionIsPlausible(t *testing.T) {
	effects := []AggregatedEventEffect{{Kind: "meal", Deltas: map[string]float64{"HUNGER": 120}}}
	a := testAgent(c("HUNGER", -80))
	a.SimulationLevel = Simplified
	ApplyAggregatedEvent(&a, AggregatedEvent{Kind: "meal", AtHour: 12})
	TransitionToFull(&a, 13, effects)
	assert.GreaterOrEqual(t, a.Considerations["HUNGER"].Value, 30.0)
}

// LOD reconstruction is data-driven: the effects table maps aggregated event
// kinds to consideration deltas, and the engine applies it without knowing
// event or consideration names. Only events that happened by the target hour
// and whose kind is declared apply.
func TestTransitionToFullAppliesDeclaredEventEffects(t *testing.T) {
	effects := []AggregatedEventEffect{{Kind: "meal", Deltas: map[string]float64{"HUNGER": 120}}}
	a := testAgent(c("HUNGER", -80))
	a.SimulationLevel = Simplified
	ApplyAggregatedEvent(&a, AggregatedEvent{Kind: "meal", AtHour: 12})
	ApplyAggregatedEvent(&a, AggregatedEvent{Kind: "meal", AtHour: 14})
	ApplyAggregatedEvent(&a, AggregatedEvent{Kind: "nap", AtHour: 12})

	TransitionToFull(&a, 13, effects)

	assert.InDelta(t, 40, a.Considerations["HUNGER"].Value, 0.0001)
	assert.Empty(t, a.AggregatedEvents)
	assert.Equal(t, Full, a.SimulationLevel)
}

// A profile without an aggregated-event table reconstructs nothing: the
// transition only discards the event log and restores full detail.
func TestTransitionToFullWithoutDeclaredEffectsReconstructsNothing(t *testing.T) {
	a := testAgent(c("HUNGER", -80))
	a.SimulationLevel = Simplified
	ApplyAggregatedEvent(&a, AggregatedEvent{Kind: "meal", AtHour: 12})

	TransitionToFull(&a, 13, nil)

	assert.InDelta(t, -80, a.Considerations["HUNGER"].Value, 0.0001)
	assert.Empty(t, a.AggregatedEvents)
	assert.Equal(t, Full, a.SimulationLevel)
}

// A player-queued action is protected from preemption unless a consideration
// reaches imminent collapse. Collapse is read from the consideration's own
// CriticalThreshold, not from an engine-wide constant: with a threshold of
// -20, a value of -25 is already a collapse and the queued action yields.
func TestPlayerQueuedActionYieldsAtConsiderationCriticalThreshold(t *testing.T) {
	critical := c("HUNGER", -25)
	critical.CriticalThreshold = -20
	a := testAgent(critical)
	a.CurrentAction = &ActionInstance{Action: action("leisure", map[string]float64{"HUNGER": 1}), ContinuationUtility: 1, PlayerQueued: true}
	ps := []AffordanceProvider{provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))}
	assert.True(t, CheckPreemption(&a, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 1))))
	assert.Equal(t, "eat", a.CurrentAction.Action.ActionID)
}

// A consideration without a configured CriticalThreshold (zero) falls back to
// the engine's named imminent-collapse floor: a player-queued action survives
// a value of -50 but yields at -95.
func TestPlayerQueuedActionFallsBackToNamedCollapseThreshold(t *testing.T) {
	queued := func(value float64) Agent {
		consideration := c("HUNGER", value)
		consideration.CriticalThreshold = 0
		a := testAgent(consideration)
		a.CurrentAction = &ActionInstance{Action: action("leisure", map[string]float64{"HUNGER": 1}), ContinuationUtility: 1, PlayerQueued: true}
		return a
	}
	ps := []AffordanceProvider{provider("food", Position{}, action("eat", map[string]float64{"HUNGER": 80}))}

	protected := queued(-50)
	assert.False(t, CheckPreemption(&protected, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 1))))

	collapsed := queued(-95)
	assert.True(t, CheckPreemption(&collapsed, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 1))))
}

// During an active NarrativeCommitment (e.g., romance, escort), the agent does
// not autonomously start actions that contradict it.
func TestNarrativeCommitmentBlocksContradictions(t *testing.T) {
	a := testAgent(c("FUN", -80))
	a.NarrativeCommitments = []NarrativeCommitment{{ID: "escort", ContradictoryTags: map[string]bool{"abandon-escort": true}}}
	ps := []AffordanceProvider{provider("bar", Position{}, action("leave", map[string]float64{"FUN": 80}, "abandon-escort"))}
	assert.Nil(t, SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(1, 2))))
}

// The same engine, with no code changes, runs the four genre configurations by
// swapping only consideration tables, providers, tags, and parameters. Loads
// the four real profiles and verifies that each defines its own considerations
// and tuning.
func TestGenreProfilesUseSameMotor(t *testing.T) {
	for _, name := range []string{"social-life", "tactical-stealth", "survival-crafting", "open-world-rpg"} {
		t.Run(name, func(t *testing.T) {
			profile, err := LoadProfile("../../configs/" + name + ".json")
			require.NoError(t, err)
			assert.NotEmpty(t, profile.Considerations)
			assert.NotZero(t, profile.Tuning.SelectionTemperature)
		})
	}
}

func TestProfilePreservesConsiderationBounds(t *testing.T) {
	profile, err := LoadProfile("../../configs/tactical-stealth.json")
	require.NoError(t, err)
	require.NotEmpty(t, profile.Considerations)
	assert.InDelta(t, -100, profile.Considerations[0].Min, 0.0001)
	assert.InDelta(t, 0, profile.Considerations[0].Value, 0.0001)
	assert.InDelta(t, 100, profile.Considerations[0].Max, 0.0001)
}

// The social-life profile declares its LOD reconstruction table as data: a
// "meal" aggregated event restores HUNGER when an agent returns to full
// detail.
func TestProfileLoadsAggregatedEventEffects(t *testing.T) {
	profile, err := LoadProfile("../../configs/social-life.json")
	require.NoError(t, err)
	require.NotEmpty(t, profile.AggregatedEventEffects)
	assert.Equal(t, "meal", profile.AggregatedEventEffects[0].Kind)
	assert.InDelta(t, 120, profile.AggregatedEventEffects[0].Deltas["HUNGER"], 0.0001)
}

// An action with deltas {+FUN 40, -ENERGY 30} is chosen by a bored, rested
// agent and avoided by an exhausted, bored agent, in at least 80% of the
// samples of each case.
func TestMixedDeltasRespondToState(t *testing.T) {
	play := action("play", map[string]float64{"FUN": 40, "ENERGY": -30})
	rest := action("rest", map[string]float64{"ENERGY": 40})
	ps := []AffordanceProvider{provider("game", Position{}, play), provider("bed", Position{}, rest)}
	enthused := testAgent(c("FUN", -90), c("ENERGY", 80))
	exhausted := testAgent(c("FUN", -90), c("ENERGY", -90))
	count := func(a Agent) int {
		n := 0
		for i := uint64(0); i < 100; i++ {
			if SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(i, 31))).Action.ActionID == "play" {
				n++
			}
		}
		return n
	}
	assert.GreaterOrEqual(t, count(enthused), 80)
	assert.LessOrEqual(t, count(exhausted), 20)
}

func TestScoreActionUsesTuningResponseCurveExponentWhenConsiderationHasNoOverride(t *testing.T) {
	hunger := c("HUNGER", 0)
	hunger.ResponseCurve.Exponent = 0
	agent := testAgent(hunger)
	tuning := DefaultTuning()
	tuning.ResponseCurveExponent = 3
	advertisedAction := action("eat", map[string]float64{"HUNGER": 1})

	utility := ScoreAction(agent, provider("food", Position{}, advertisedAction), advertisedAction, tuning)

	assert.InDelta(t, 0.125, utility, 0.0001)
}

func TestScoreActionUsesConsiderationExponentInsteadOfTuningDefault(t *testing.T) {
	hunger := c("HUNGER", 0)
	hunger.ResponseCurve.Exponent = 4
	agent := testAgent(hunger)
	tuning := DefaultTuning()
	tuning.ResponseCurveExponent = 3
	advertisedAction := action("eat", map[string]float64{"HUNGER": 1})

	utility := ScoreAction(agent, provider("food", Position{}, advertisedAction), advertisedAction, tuning)

	assert.InDelta(t, 0.0625, utility, 0.0001)
}

func TestDeterminismClampAndResponseCurves(t *testing.T) {
	for _, tc := range []struct {
		name        string
		curve       ResponseCurve
		value, want float64
	}{{"convex", ResponseCurve{Kind: Convex, Exponent: 2}, 0, .25}, {"linear", ResponseCurve{Kind: Linear}, 0, .5}, {"step", ResponseCurve{Kind: Step, Threshold: 0, Below: 1, Above: .2}, -1, 1}, {"logistic", ResponseCurve{Kind: Logistic, Slope: 1, Midpoint: 0}, 0, .5}} {
		t.Run(tc.name, func(t *testing.T) { assert.InDelta(t, tc.want, tc.curve.Evaluate(tc.value, -100, 100), .0001) })
	}
	decay := c("HUNGER", 99)
	decay.Updater = ConsiderationUpdater{Kind: LinearRegen, Rate: 100}
	a := testAgent(decay)
	UpdateConsiderations(&a, World{}, 1)
	assert.InDelta(t, 100, a.Considerations["HUNGER"].Value, .0001)
	ps := []AffordanceProvider{provider("p", Position{}, action("eat", map[string]float64{"HUNGER": 20}))}
	one := SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(4, 5)))
	two := SelectAction(a, ps, DefaultTuning(), rand.New(rand.NewPCG(4, 5)))
	require.NotNil(t, one)
	assert.Equal(t, one.Action.ActionID, two.Action.ActionID)
}
