# specs

<!-- okfw:generated — maintained by okfw; edits here are overwritten -->

## Concepts

* [On-behalf-of requests](on-behalf-of-requests--01a1f5379dea.md) - An allowlisted machine actor sends X-On-Behalf-Of on an allowlisted route to act as a person, and chitd provisions a person it has not seen.
* [REST API contract](rest-api-contract--01a1f5379dda.md) - api/openapi.yaml is the contract of the chitd REST API under /api/v1; the router and the spec change together.
* [Slash commands](slash-commands--01a1f5379de5.md) - CUE files define the slash commands, Keto authorizes each command, and chitd audits each one and replies with a command_response event.
* [WebSocket events](websocket-events--01a1f5379de0.md) - chitd sends 15 event types over /api/v1/websocket to the members of the channel or team of each event.

