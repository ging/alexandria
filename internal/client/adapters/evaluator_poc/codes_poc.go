// Package evaluator_poc declares the PoC-only info code of evaluator_poc, outside the closed F5 catalog and marked as such (DP-7).
// Emitted once per ignored SHACL or JSON-LD artefact (REQ-CLI-010).
package evaluator_poc

import "github.com/ging/alexandria/internal/core/codes"

// PocEvalUnsupportedRole reports an artefact role the PoC evaluator does not run (not in F5).
const PocEvalUnsupportedRole codes.Code = "POC_EVAL_UNSUPPORTED_ROLE"
