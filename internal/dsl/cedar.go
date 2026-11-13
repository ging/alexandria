// Package dsl gates the Cedar projection of a profile behind one switch while that integration is disconnected.
// With cedarEnabled false no Cedar artefact, role or annotation is produced; enforcement blocks are still parsed and validated.
package dsl

import "strings"

// cedarEnabled turns the Cedar projection on: cedar-mapping.json, cedar.schema, their roles and alx:cedarPath.
var cedarEnabled = false

// emitsCedar reports whether a profile produces Cedar artefacts.
func emitsCedar(ix *Index) bool { return cedarEnabled && hasEnforcement(ix) }

// declarations returns the OWL annotation declarations, without the Cedar property while Cedar is disconnected.
func declarations() string {
	if cedarEnabled {
		return annotationDeclarations
	}
	return strings.Replace(annotationDeclarations, "alx:cedarPath a owl:AnnotationProperty .\n", "", 1)
}
