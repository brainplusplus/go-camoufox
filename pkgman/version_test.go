package pkgman

import "testing"

func TestVersionComparePythonStyleBuilds(t *testing.T) {
	if (Version{Build: "alpha.1"}).Compare(Version{Build: "beta.19"}) >= 0 {
		t.Fatal("expected alpha.1 < beta.19")
	}
	if (Version{Build: "beta.25"}).Compare(Version{Build: "beta.19"}) <= 0 {
		t.Fatal("expected beta.25 > beta.19")
	}
	if !(Version{Build: "beta.25"}).IsSupported() {
		t.Fatal("expected beta.25 to be supported")
	}
	if (Version{Build: "1"}).IsSupported() {
		t.Fatal("expected 1 to be outside max constraint")
	}
}
