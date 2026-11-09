// SemVer 2.0.0 parsing and precedence ordering for versions of a SpaceProfile.
// Both binaries order versions with this single implementation (§2.5, REQ-SYS-001).
package semver

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var re = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

// Version is a comparable SemVer value; Pre and Build keep their dotted text.
type Version struct {
	Major, Minor, Patch uint64
	Pre, Build          string
}

// Parse parses a SemVer 2.0.0 string.
func Parse(s string) (Version, error) {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("semver: %q is not a SemVer 2.0.0 version", s)
	}
	var v Version
	v.Major, _ = strconv.ParseUint(m[1], 10, 64)
	v.Minor, _ = strconv.ParseUint(m[2], 10, 64)
	v.Patch, _ = strconv.ParseUint(m[3], 10, 64)
	v.Pre, v.Build = m[4], m[5]
	return v, nil
}

// MustParse parses a version and panics on error; intended for fixtures.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// String renders the version in SemVer form.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare returns -1, 0 or 1 following SemVer precedence (build metadata is ignored).
func Compare(a, b Version) int {
	for _, p := range [][2]uint64{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if p[0] != p[1] {
			if p[0] < p[1] {
				return -1
			}
			return 1
		}
	}
	return comparePre(a.Pre, b.Pre)
}

// comparePre orders pre-release identifiers; a version without pre-release ranks higher.
func comparePre(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdent(as[i], bs[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// compareIdent compares one pre-release identifier: numeric ones rank below alphanumeric ones.
func compareIdent(a, b string) int {
	an, aerr := strconv.ParseUint(a, 10, 64)
	bn, berr := strconv.ParseUint(b, 10, 64)
	switch {
	case aerr == nil && berr == nil:
		if an == bn {
			return 0
		}
		if an < bn {
			return -1
		}
		return 1
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// Less reports whether a precedes b.
func Less(a, b Version) bool { return Compare(a, b) < 0 }

// MarshalJSON encodes the version as a string.
func (v Version) MarshalJSON() ([]byte, error) { return json.Marshal(v.String()) }

// UnmarshalJSON decodes a SemVer string.
func (v *Version) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := Parse(s)
	if err != nil {
		return err
	}
	*v = p
	return nil
}
