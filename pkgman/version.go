package pkgman

import (
	"strconv"
	"strings"
)

const (
	MinBuild = "alpha.1"
	MaxBuild = "1"
)

type Version struct {
	Build   string `json:"build"`
	Version string `json:"version,omitempty"`
}

func NewVersion(build, version string) Version {
	return Version{Build: build, Version: version}
}

func (v Version) FullString() string {
	if v.Version == "" {
		return v.Build
	}
	return v.Version + "-" + v.Build
}

func (v Version) Compare(other Version) int {
	left := sortableBuild(v.Build)
	right := sortableBuild(other.Build)
	for i := 0; i < len(left) || i < len(right); i++ {
		a, b := 0, 0
		if i < len(left) {
			a = left[i]
		}
		if i < len(right) {
			b = right[i]
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	}
	return 0
}

func (v Version) IsSupported() bool {
	return v.Compare(Version{Build: MinBuild}) >= 0 && v.Compare(Version{Build: MaxBuild}) < 0
}

func VersionFromPath(path string) (Version, error) {
	metadata, err := ReadVersionMetadata(path)
	if err != nil {
		return Version{}, err
	}
	return Version{Build: metadata.Build, Version: metadata.Version}, nil
}

func BuildMinMax() (Version, Version) {
	return Version{Build: MinBuild}, Version{Build: MaxBuild}
}

func sortableBuild(build string) []int {
	parts := strings.Split(strings.TrimLeft(build, "^~"), ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if number, err := strconv.Atoi(part); err == nil {
			out = append(out, number)
			continue
		}
		if part == "" {
			out = append(out, 0)
			continue
		}
		out = append(out, int([]rune(part)[0])-1024)
	}
	return out
}
