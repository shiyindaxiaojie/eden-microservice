# Eden Microservice Context

Read this compact glossary only when a task uses unfamiliar or ambiguous domain terms.

Registry: service registration, discovery, health, topology, and events.
Config center: versioned configuration resources, history, and watches; not process configuration.
Gateway control plane: route definitions, validation, publication, and admin APIs.
Gateway data plane: proxy matching, load balancing, and filters.
Compatibility adapter: Nacos or Consul protocol boundary, not native domain model.

Source of truth: external system that owns runtime state; local storage records synchronized control-plane state.
Console contract: backend API, client, types, routes, and view evolve together.

Add only confirmed reusable terms. Keep behavior rules in `specs/`, not here.
