package initializr

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// Supports reports whether the dependency can be used with the given Spring
// Boot version. A dependency without a version range works with every
// version. When the range or the version cannot be read, the answer is yes:
// Initializr checks the combination again when it generates the project.
func (d Dependency) Supports(bootVersion string) bool {
	versionRange := strings.TrimSpace(d.VersionRange)
	if versionRange == "" || bootVersion == "" {
		return true
	}
	version, ok := parseVersion(bootVersion)
	if !ok {
		return true
	}

	// A single version means that version or any later one.
	lower, upper, bounded := strings.Cut(versionRange, ",")
	if !bounded {
		from, ok := parseVersion(versionRange)
		return !ok || compareVersions(version, from) >= 0
	}

	// "[4.0.0,4.2.0-M1)": a square bracket includes its end, a round one
	// leaves it out.
	if len(lower) < 2 || len(upper) < 2 {
		return true
	}
	from, okFrom := parseVersion(lower[1:])
	to, okTo := parseVersion(upper[:len(upper)-1])
	open, closing := lower[0], upper[len(upper)-1]
	if !okFrom || !okTo || !strings.ContainsRune("[(", rune(open)) || !strings.ContainsRune("])", rune(closing)) {
		return true
	}

	afterStart, beforeEnd := compareVersions(version, from), compareVersions(version, to)
	return (afterStart > 0 || (afterStart == 0 && open == '[')) &&
		(beforeEnd < 0 || (beforeEnd == 0 && closing == ']'))
}

// Pre-release stages in the order they lead up to a release.
const (
	stageMilestone = iota
	stageReleaseCandidate
	stageSnapshot
	stageRelease
)

// A version is major, minor, patch, stage and the number of the milestone or
// release candidate, e.g. 4.2.0-M2 is {4, 2, 0, stageMilestone, 2}.
type version [5]int

func compareVersions(a, b version) int {
	return slices.CompareFunc(a[:], b[:], cmp.Compare[int])
}

// parseVersion reads a version such as "4.1.1", "4.2.0-M2", "4.2.0-RC1" or
// "4.2.0-SNAPSHOT".
func parseVersion(s string) (version, bool) {
	numbers, qualifier, qualified := strings.Cut(strings.TrimSpace(s), "-")

	v := version{3: stageRelease}
	parts := strings.Split(numbers, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, false
		}
		v[i] = n
	}
	if !qualified {
		return v, true
	}

	prefix := "RC"
	switch {
	case qualifier == "SNAPSHOT":
		v[3] = stageSnapshot
		return v, true
	case strings.HasPrefix(qualifier, "RC"):
		v[3] = stageReleaseCandidate
	case strings.HasPrefix(qualifier, "M"):
		v[3], prefix = stageMilestone, "M"
	default:
		return version{}, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(qualifier, prefix))
	if err != nil || n < 0 {
		return version{}, false
	}
	v[4] = n
	return v, true
}
