# Compress Building

Subdomain: compress/
Source paths: internal/compress/

Three-stage token compression pipeline. Each strategy is independently toggleable via config. The pipeline runs dedup → structural collapse → relevance trimming in order; early exits prevent double-processing.

## TASK → LOAD

| Task | Load |
|------|------|
| Understand compression pipeline entry point and ordering | core.md |
| Add a new output format / shape detector | core.md |
| Debug dedup false positive or near-match behavior | core.md |
| Tune TF-IDF relevance trimming or token budget | core.md |
| Understand how strategies are selected and combined | core.md |
| Add a new error keyword or change passthrough behavior | core.md |
| Understand EstimateTokens heuristic | core.md |

## Rooms

| Room | Source paths | Files |
|------|-------------|-------|
| core.md | internal/compress/ | pipeline.go, dedup.go, structure.go, relevance.go |
