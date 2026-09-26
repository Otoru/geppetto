// Package server adapts the protobuf wire contract to the stateless engine.
// It loads decision profiles once at startup, decodes homogeneous requests,
// executes the scoring phase in a bounded worker pool, reconciles contested
// slots, and encodes one result per input agent. It does not retain NPC state
// between requests and does not implement the engine's scoring rules itself.
//
// BatchDecide is the production path. A request names exactly one profile, so
// the order of that profile's considerations defines the stride and meaning
// of every agent's consideration_values array. Mixing profiles in a batch
// would make the same offset mean different things and is therefore not
// supported. Decide is a one-agent convenience endpoint implemented by
// forwarding through the same batch path, which keeps debugging and tuning
// behavior aligned with production.
//
// The batch layout is structure-of-arrays (SoA). Agent IDs, each coordinate,
// consideration values, provider coordinates, capacities, and action fields
// travel in parallel arrays; offset arrays delimit each action's tags and
// consideration deltas. This avoids a repeated Agent envelope and its per-
// object serialization and allocation overhead. At 50,000 or more agents per
// tick, replacing the layout with one message per agent would put wire and GC
// overhead back inside the decision budget. The server validates all array
// lengths and offsets before any worker starts scoring.
//
// ProfileCache is populated from JSON files once during process startup. A
// request only performs an in-memory profile lookup; it never performs
// profile file I/O. Each rankOne call obtains a reusable engine.Agent from a
// sync.Pool, fills it with the request's position and profile-ordered values,
// derives a per-agent pseudo-random stream from the batch seed and index, and
// returns the temporary object to the pool after producing its preference
// list.
//
// BatchDecide has two deliberately separate phases. In phase one, up to
// runtime.GOMAXPROCS workers score agents independently. Providers are
// read-only, and each worker writes only its own preference-list slot, so no
// decision lock is needed. In phase two, ResolveContention runs a deterministic
// serial reconciliation. It gives contested provider and action slots to the
// nearest agents, lets losers walk their own bounded fallback lists, and
// leaves the response's selected_action_index at -1 when no candidate remains.
// There is one debug log entry per batch rather than one log entry per agent;
// that keeps observability from becoming a 50,000-agent allocation and I/O
// cost. The request context can cancel work between jobs and is checked before
// the response is returned.
//
// The wire contract has an important intentional limit. The engine model can
// express agent capabilities, resource costs, provider state, and provider
// occupants, and the standalone engine checks those richer eligibility rules.
// The current BatchDecide payload does not transport preconditions, cost,
// provider state, or provider occupants. It transports agent identity,
// position, and consideration values; provider identity, position, and
// capacity; and action identity, duration, tags, domain, priority, radius,
// deltas, action capacity, and action occupancy. Consequently, on the served
// batch path, effective eligibility is reduced to reach and available slots at
// the provider and action levels. A client that needs capability, inventory,
// cost, or provider-state filtering must apply it before constructing the
// announcement (and may need separate batches when eligibility differs by
// agent). This is a contract boundary, not an accidental omission, and it is
// why adding those semantics later is a transport extension rather than a
// reinterpretation of the existing score.
//
// Provider capacity zero blocks a provider. Action capacity zero means no
// action-level limit, including when the action-capacity array is omitted for
// compatibility with older proto3 clients. Positive action capacities and
// action occupancies are checked per action and per provider instance; the
// stricter provider/action level wins. The list of provider occupants is not
// in the batch, so provider slots begin from the capacity supplied by the
// client and are consumed only by assignments made within that call.
package server
