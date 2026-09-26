package geppetto

import "math"

const (
	// Default calibration is intentionally private: callers configure the
	// exported Tuning fields or use DefaultTuning rather than depending on
	// individual calibration constants.
	wPriority            = 5.0
	distanceReference    = 10.0
	selectionTopK        = 3
	selectionTemperature = 1.0
	// reconciliationTopK is the default number of ordered candidates each
	// agent retains for contention resolution. At 50k agents per batch,
	// retaining full candidate lists per agent would blow the memory budget.
	reconciliationTopK = 5
	// responseCurveExponentDefault is the default exponent for convex response
	// curves without a per-consideration or per-profile override.
	responseCurveExponentDefault = 2.0
	// preemptionMarginDefault is the default utility factor a candidate must
	// strictly exceed to interrupt the current action.
	preemptionMarginDefault = 1.5
	// imminentCollapseThreshold is the fallback collapse floor for a
	// consideration without a configured CriticalThreshold.
	imminentCollapseThreshold  = -90.0
	fullTickHoursDefault       = 1.0 / 60
	simplifiedTickHoursDefault = 1.0
)

// Position is a three-dimensional point in world units.
type Position struct{ X, Y, Z float64 }

// Distance returns the Euclidean distance from p to to.
func (p Position) Distance(to Position) float64 {
	return math.Sqrt((p.X-to.X)*(p.X-to.X) + (p.Y-to.Y)*(p.Y-to.Y) + (p.Z-to.Z)*(p.Z-to.Z))
}

// ResponseCurveKind identifies the function used to turn a consideration
// value into a normalized pressure.
type ResponseCurveKind string

const (
	// Convex increases pressure rapidly as a consideration approaches its minimum.
	Convex ResponseCurveKind = "convex"
	// Linear changes pressure proportionally to the consideration value.
	Linear ResponseCurveKind = "linear"
	// Step switches pressure at the curve threshold.
	Step ResponseCurveKind = "step"
	// Logistic transitions pressure smoothly around the curve midpoint.
	Logistic ResponseCurveKind = "logistic"
)

// ResponseCurve configures the response-curve parameters for a Consideration.
type ResponseCurve struct {
	Kind ResponseCurveKind `json:"kind"`
	// Exponent overrides the profile default for this convex consideration.
	Exponent  float64 `json:"exponent,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Below     float64 `json:"below,omitempty"`
	Above     float64 `json:"above,omitempty"`
	Slope     float64 `json:"slope,omitempty"`
	Midpoint  float64 `json:"midpoint,omitempty"`
}

// UpdaterKind identifies how a Consideration changes between decisions.
type UpdaterKind string

const (
	// LinearDecay subtracts Rate on every update.
	LinearDecay UpdaterKind = "linear_decay"
	// LinearRegen adds Rate on every update.
	LinearRegen UpdaterKind = "linear_regen"
	// EventDriven changes only when an explicit event mutates it.
	EventDriven UpdaterKind = "event_driven"
	// PerceptionDriven reads a value from the perceived world.
	PerceptionDriven UpdaterKind = "perception_driven"
	// RelationshipDriven derives its value from a relationship.
	RelationshipDriven UpdaterKind = "relationship_driven"
	// ContextAggregate derives its value from environmental context.
	ContextAggregate UpdaterKind = "context_aggregate"
)

// World supplies external values used by consideration updaters. Perception,
// Relationships, and ContextValues feed their corresponding updater kinds. It
// is input to a tick, not state owned by the engine.
type World struct {
	Perception    map[string]float64
	Relationships map[string]float64
	ContextValues map[string]float64
}

// ConsiderationUpdater defines the dynamics applied to a Consideration.
type ConsiderationUpdater struct {
	Kind  UpdaterKind                `json:"kind"`
	Rate  float64                    `json:"rate,omitempty"`
	Value func(Agent, World) float64 `json:"-"`
}

// Consideration is a normalized NPC signal, its bounds, and its scoring rule.
type Consideration struct {
	ID                string               `json:"id"`
	Value             float64              `json:"value"`
	Min               float64              `json:"min"`
	Max               float64              `json:"max"`
	Updater           ConsiderationUpdater `json:"updater"`
	BaseWeight        float64              `json:"base_weight"`
	CriticalThreshold float64              `json:"critical_threshold"`
	ResponseCurve     ResponseCurve        `json:"response_curve"`
}

// Profile defines the consideration order and tuning shared by a decision
// batch or simulation. AggregatedEventEffects rebuild detailed state after a
// simplified simulation interval.
type Profile struct {
	Name                   string                  `json:"name"`
	Considerations         []Consideration         `json:"considerations"`
	Tuning                 Tuning                  `json:"tuning"`
	AggregatedEventEffects []AggregatedEventEffect `json:"aggregated_event_effects,omitempty"`
}

// Trait changes action-tag multipliers and consideration updater rates.
type Trait struct {
	ID                 string             `json:"id"`
	Modifiers          map[string]float64 `json:"modifiers"`
	ConsiderationDelta map[string]float64 `json:"consideration_delta"`
}

// Personality collects an agent's domain preferences and traits.
type Personality struct {
	Preferences map[string]float64 `json:"preferences"`
	Traits      []Trait            `json:"traits"`
}

// Context temporarily biases actions carrying matching tags.
type Context struct {
	Modifiers map[string]float64 `json:"modifiers"`
	Duration  float64            `json:"duration"`
}

// SimulationLevel identifies whether the agent receives detailed or aggregated updates.
type SimulationLevel string

const (
	// Full runs the detailed per-tick decision loop.
	Full SimulationLevel = "FULL"
	// Simplified represents an agent with aggregated, low-detail events.
	Simplified SimulationLevel = "SIMPLIFIED"
)

// NarrativeCommitment prevents autonomous actions with contradictory tags.
// ExpiresAt is an absolute simulation hour; zero means the commitment does not expire.
type NarrativeCommitment struct {
	ID                string          `json:"id"`
	ContradictoryTags map[string]bool `json:"contradictory_tags"`
	ExpiresAt         float64         `json:"expires_at"`
}

// Agent contains all state the engine needs to update and score one NPC.
type Agent struct {
	ID             string
	Considerations map[string]Consideration
	Personality    Personality
	CurrentAction  *ActionInstance
	// ActionQueue holds FIFO actions supplied through EnqueueAction. The engine
	// starts its head whenever the agent becomes idle.
	ActionQueue          []ActionInstance
	Position             Position
	Capabilities         map[string]bool
	ActiveContexts       []Context
	SimulationLevel      SimulationLevel
	NarrativeCommitments []NarrativeCommitment
	Resources            map[string]float64
	AggregatedEvents     []AggregatedEvent
}

// Precondition is an agent capability/resource requirement or a provider-state
// requirement for an AdvertisedAction.
type Precondition struct {
	Capability string  `json:"capability,omitempty"`
	Resource   string  `json:"resource,omitempty"`
	Minimum    float64 `json:"minimum,omitempty"`
	// RequiredState must match the AffordanceProvider state when it is non-empty.
	RequiredState string `json:"required_state,omitempty"`
}

// AdvertisedAction is a provider's declarative promise of consideration deltas.
type AdvertisedAction struct {
	ActionID            string             `json:"action_id"`
	Deltas              map[string]float64 `json:"deltas"`
	EstimatedDuration   float64            `json:"estimated_duration"`
	Tags                []string           `json:"tags"`
	Domain              string             `json:"domain"`
	Preconditions       []Precondition     `json:"preconditions"`
	IntrinsicPriority   float64            `json:"intrinsic_priority"`
	AdvertisementRadius float64            `json:"advertisement_radius"`
	Cost                map[string]float64 `json:"cost"`
	// Capacity caps simultaneous agents on THIS action. WARNING: unlike
	// AffordanceProvider.Capacity, zero means UNLIMITED — proto3 encodes an
	// absent numeric field as 0 and every pre-existing client sends 0, so 0
	// must read as "no own limit" or every existing action would brick. An
	// agent joins only with a free slot at both levels; the stricter wins.
	Capacity int `json:"capacity"`
	// Occupancy is how many agents the client reports as already executing
	// this action at tick start. It consumes action slots before the batch
	// is reconciled.
	Occupancy int `json:"occupancy"`
}

// AffordanceProvider advertises actions at a world position with limited capacity.
type AffordanceProvider struct {
	ID                string             `json:"id"`
	Position          Position           `json:"position"`
	AdvertisedActions []AdvertisedAction `json:"advertised_actions"`
	Capacity          int                `json:"capacity"`
	Occupants         []string           `json:"occupants"`
	State             string             `json:"state"`
}

// ActionInstance records a selected action and its progress for preemption.
type ActionInstance struct {
	Action              AdvertisedAction
	ProviderID          string
	Elapsed             float64
	ContinuationUtility float64
	PlayerQueued        bool
}

// Candidate couples an eligible advertised action with its scored utility.
type Candidate struct {
	Action   AdvertisedAction
	Provider AffordanceProvider
	Utility  float64
}

// Tuning collects the profile-level parameters that control engine behavior.
type Tuning struct {
	// ResponseCurveExponent is the profile default for convex curves without an override.
	ResponseCurveExponent float64 `json:"response_curve_exponent"`
	WPriority             float64 `json:"W_PRIORITY"`
	DistanceReference     float64 `json:"DISTANCE_REFERENCE"`
	SelectionTopK         int     `json:"SELECTION_TOP_K"`
	SelectionTemperature  float64 `json:"SELECTION_TEMPERATURE"`
	// ReconciliationTopK caps how many ordered candidates each agent retains
	// for contention resolution: the stochastic first pick plus
	// utility-ordered fallbacks.
	ReconciliationTopK int     `json:"RECONCILIATION_TOP_K"`
	PreemptionMargin   float64 `json:"preemption_margin"`
	// ConventionBreakProbability replaces the normal choice with a strictly
	// lower-utility eligible action when its probability succeeds.
	ConventionBreakProbability float64 `json:"convention_break_probability"`
	// PerceptionNoise is the standard deviation of Gaussian noise applied to
	// perception-driven considerations while scoring a decision.
	PerceptionNoise float64 `json:"perception_noise"`
	// FullTickHours is the simulated duration of one detailed Advance call.
	FullTickHours float64 `json:"full_tick_hours"`
	// SimplifiedTickHours is the simulated duration of one aggregated Advance call.
	SimplifiedTickHours float64 `json:"simplified_tick_hours"`
}

// DefaultTuning returns the specification's default tuning values.
func DefaultTuning() Tuning {
	return Tuning{
		ResponseCurveExponent:      responseCurveExponentDefault,
		WPriority:                  wPriority,
		DistanceReference:          distanceReference,
		SelectionTopK:              selectionTopK,
		SelectionTemperature:       selectionTemperature,
		ReconciliationTopK:         reconciliationTopK,
		PreemptionMargin:           preemptionMarginDefault,
		ConventionBreakProbability: 0,
		PerceptionNoise:            0,
		FullTickHours:              fullTickHoursDefault,
		SimplifiedTickHours:        simplifiedTickHoursDefault,
	}
}

// AggregatedEvent records a coarse simulation event for a simplified agent.
type AggregatedEvent struct {
	Kind   string
	AtHour float64
}

// AggregatedEventEffect declares how one kind of aggregated event rebuilds
// consideration state when an agent returns to full simulation detail. The
// profile owns the table; the engine applies it without knowing event or
// consideration names.
type AggregatedEventEffect struct {
	// Kind matches AggregatedEvent.Kind.
	Kind string `json:"kind"`
	// Deltas are added to the matching considerations, clamped to their bounds.
	Deltas map[string]float64 `json:"deltas"`
}
