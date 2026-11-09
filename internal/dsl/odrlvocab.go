// Package dsl holds a local copy of the ODRL 2.2 Core and Common Vocabulary terms by category, used by N3 to check adopted terms.
// Adopted terms are checked against this copy and never against the network (§8.4, §12 N3).
package dsl

// odrlVocab maps an ODRL local name to its category in the Vocabulary & Expression 2.2.
var odrlVocab = map[string]string{}

// odrlCore lists the two actions of ODRL Core, which a profile can ignore but not forbid (Best Practice §5).
var odrlCore = []string{"use", "transfer"}

// odrlOperators lists the twelve ODRL operators.
var odrlOperators = []string{"eq", "neq", "lt", "lteq", "gt", "gteq", "isA", "hasPart", "isPartOf", "isAllOf", "isAnyOf", "isNoneOf"}

func init() {
	add := func(kind string, names ...string) {
		for _, n := range names {
			odrlVocab[n] = kind
		}
	}
	add("action", "use", "transfer", "acceptTracking", "aggregate", "annotate", "anonymize", "archive", "attribute",
		"compensate", "concurrentUse", "delete", "derive", "digitize", "display", "distribute", "ensureExclusivity",
		"execute", "extract", "give", "grantUse", "include", "index", "inform", "install", "modify", "move",
		"nextPolicy", "obtainConsent", "play", "present", "print", "read", "reproduce", "reviewPolicy", "sell",
		"stream", "synchronize", "textToSpeech", "transform", "translate", "uninstall", "watermark",
		"append", "appendTo", "copy", "export", "lease", "lend", "license", "pay", "secondaryUse", "write", "writeTo")
	add("left_operand", "absolutePosition", "absoluteSpatialPosition", "absoluteTemporalPosition", "absoluteSize",
		"count", "dateTime", "delayPeriod", "deliveryChannel", "elapsedTime", "event", "fileFormat", "industry",
		"language", "media", "meteredTime", "payAmount", "percentage", "product", "purpose", "recipient",
		"relativePosition", "relativeSpatialPosition", "relativeTemporalPosition", "relativeSize", "resolution",
		"spatial", "spatialCoordinates", "systemDevice", "timeInterval", "unitOfCount", "version", "virtualLocation",
		"device", "system")
	add("operator", odrlOperators...)
	add("right_operand", "policyUsage")
	add("party_function", "assigner", "assignee", "attributedParty", "attributingParty", "compensatedParty",
		"compensatingParty", "consentingParty", "consentedParty", "contractedParty", "contractingParty",
		"informedParty", "informingParty", "trackingParty", "trackedParty")
	add("asset_relation", "target", "output")
}

// categoryKind maps an adopted category of §8.4 to a term kind of §8.5.
var categoryKind = map[string]string{
	"actions": "action", "left_operands": "left_operand", "operators": "operator",
	"right_operands": "right_operand", "party_functions": "party_function", "asset_relations": "asset_relation",
}
