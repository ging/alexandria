# Runs the same compact ODRL offers through the generated schema.json and shapes.ttl and prints both outcomes side by side.
import json, sys, copy
from jsonschema import Draft202012Validator
from rdflib import Graph
from pyshacl import validate

OUT = sys.argv[1]
CTX_ODRL = {  # minimal offline stand-in for the ODRL 2.2 JSON-LD context
    "odrl": "http://www.w3.org/ns/odrl/2/", "xsd": "http://www.w3.org/2001/XMLSchema#",
    "Offer": "odrl:Offer", "uid": "@id", "type": "@type",
    "profile": {"@id": "odrl:profile", "@type": "@id"},
    "permission": {"@id": "odrl:permission", "@type": "@id"},
    "prohibition": {"@id": "odrl:prohibition", "@type": "@id"},
    "obligation": {"@id": "odrl:obligation", "@type": "@id"},
    "duty": {"@id": "odrl:duty", "@type": "@id"},
    "target": {"@id": "odrl:target", "@type": "@id"},
    "action": {"@id": "odrl:action", "@type": "@vocab"},
    "constraint": {"@id": "odrl:constraint", "@type": "@id"},
    "leftOperand": {"@id": "odrl:leftOperand", "@type": "@vocab"},
    "operator": {"@id": "odrl:operator", "@type": "@vocab"},
    "rightOperand": "odrl:rightOperand",
    "rightOperandReference": {"@id": "odrl:rightOperandReference", "@type": "@id"},
    "and": {"@id": "odrl:and", "@type": "@id"}, "or": {"@id": "odrl:or", "@type": "@id"},
    "use": "odrl:use", "transfer": "odrl:transfer", "sell": "odrl:sell",
    **{o: "odrl:" + o for o in ["eq","neq","lt","lteq","gt","gteq","isA","hasPart","isPartOf","isAllOf","isAnyOf","isNoneOf"]},
}
profile_ctx = json.load(open(OUT + "/context.jsonld"))["@context"]
schema = Draft202012Validator(json.load(open(OUT + "/schema.json")))
shapes = Graph().parse(OUT + "/shapes.ttl", format="turtle")
PROFILE = json.load(open(OUT + "/schema.json"))["properties"]["profile"]["anyOf"][0]["enum"][0]

def offer(*perms, profile=True, **extra):
    o = {"@type": "Offer", "@id": "urn:uuid:offer-1", "target": "urn:uuid:dataset-qh-2026", "permission": list(perms), **extra}
    if profile:
        o["profile"] = PROFILE
    return o

def perm(action, *cons):
    p = {"action": action}
    if cons:
        p["constraint"] = list(cons)
    return p

def c(lo, op, ro):
    return {"leftOperand": lo, "operator": op, "rightOperand": ro}

def run(name, doc):
    errs = sorted(e.message[:70] for e in schema.iter_errors(doc))
    g = Graph().parse(data=json.dumps({"@context": [CTX_ODRL, profile_ctx], **doc}), format="json-ld")
    conforms, rg, _ = validate(g, shacl_graph=shapes, inference="none", advanced=False)
    q = """PREFIX sh: <http://www.w3.org/ns/shacl#>
    SELECT ?sev ?shape ?msg WHERE { ?r a sh:ValidationResult ; sh:resultSeverity ?sev ; sh:sourceShape ?shape . OPTIONAL { ?r sh:resultMessage ?msg } }"""
    res = sorted({(str(s).split('#')[-1], str(sh).split('#')[-1][:22] if '#' in str(sh) else 'bnode', str(m)[:60]) for s, sh, m in rg.query(q)})
    viol = [r for r in res if r[0] == "Violation"]; warn = [r for r in res if r[0] == "Warning"]
    print(f"{name:44s} schema={'ok ' if not errs else 'FAIL'}  shacl={'ok ' if not viol else 'FAIL'}{' +warn' if warn else ''}  conforms={conforms}")
    if "-v" in sys.argv:
        for e in errs: print("      schema:", e)
        for r in res: print("      shacl :", r)

cases = json.load(open(sys.argv[2])) if len(sys.argv) > 2 and not sys.argv[2].startswith("-") else None

good = perm("forecastDemand", c("purpose", "eq", {"@id": "enr:demandForecast"}), c("minAggregation", "gteq", 50), c("retentionDays", "lteq", 90))
run("01 valid offer", offer(good))
run("02 action outside the profile (sell)", offer(perm("sell")))
run("03 operator not in signature (minAgg lt)", offer(perm("forecastDemand", c("minAggregation", "lt", 50))))
run("04 wrong literal type (minAgg 'fifty')", offer(perm("forecastDemand", c("minAggregation", "gteq", "fifty"))))
run("05 purpose value outside the list", offer(perm("forecastDemand", c("purpose", "eq", {"@id": "enr:marketing"}))))
run("06 isAnyOf with two regions", offer(perm("settleImbalance", c("processingRegion", "isAnyOf", [{"@id": "enr:regionES"}, {"@id": "enr:regionEU"}]))))
run("07 ODRL Core action (use)", offer(perm("use")))
run("08 profile not declared", offer(good, profile=False))
run("09 logical constraint (and)", offer({"action": "forecastDemand", "constraint": [{"and": [c("minAggregation", "gteq", 50), c("retentionDays", "lteq", 90)]}]}))
run("10 logical constraint with a bad leaf", offer({"action": "forecastDemand", "constraint": [{"and": [c("minAggregation", "gteq", 50), c("retentionDays", "gt", 90)]}]}))
run("11 dateTime as plain string", offer(perm("forecastDemand", c("dateTime", "lt", "2027-01-01T00:00:00Z"))))
run("12 dateTime typed", offer(perm("forecastDemand", c("dateTime", "lt", {"@value": "2027-01-01T00:00:00Z", "@type": "xsd:dateTime"}))))
run("13 integer as typed value", offer(perm("forecastDemand", c("minAggregation", "gteq", {"@value": "50", "@type": "xsd:integer"}))))
run("14 decimal where integer expected", offer(perm("forecastDemand", c("minAggregation", "gteq", 50.5))))
run("15 rightOperandReference not allowed", offer(perm("forecastDemand", {"leftOperand": "minAggregation", "operator": "gteq", "rightOperandReference": "urn:x:ref"})))
run("16 left operand outside the profile", offer(perm("forecastDemand", c("odrl:spatial", "eq", {"@id": "enr:regionES"}))))
run("17 two values without a set operator", offer(perm("forecastDemand", c("retentionDays", "lteq", [30, 90]))))
run("18 eq with two purposes", offer(perm("forecastDemand", c("purpose", "eq", [{"@id": "enr:billing"}, {"@id": "enr:demandForecast"}]))))
run("19 purpose as string instead of @id", offer(perm("forecastDemand", c("purpose", "eq", "enr:billing"))))
run("20 duty with action outside the profile", offer({"action": "forecastDemand", "duty": [{"action": "sell"}]}))
run("21 action as object with @id", offer(perm({"@id": "enr:forecastDemand"})))
run("22 invalid dateTime lexical form", offer(perm("forecastDemand", c("dateTime", "lt", {"@value": "mañana", "@type": "xsd:dateTime"}))))
run("23 prohibition of profileConsumer", offer(good, prohibition=[{"action": "profileConsumer"}]))
run("26 permission with profileConsumer", offer(perm("profileConsumer")))
run("27 purpose research (new in 1.1.0)", offer(perm("forecastDemand", c("purpose", "eq", {"@id": "enr:research"}))))
run("24 rule without action", offer({"constraint": [c("minAggregation", "gteq", 50)]}))
run("25 typed integer with bad lexical form", offer(perm("forecastDemand", c("minAggregation", "gteq", {"@value": "abc", "@type": "xsd:integer"}))))
