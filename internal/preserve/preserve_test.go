package preserve

import "testing"

func TestIdenticalIsLowRisk(t *testing.T) {
	s := "Patients must not take more than 4 tablets in 24 hours (WHO, 2021)."
	r := Compare(s, s)
	if r.Risk != "low" || r.LexicalCosine != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestMeaningFlipsDetected(t *testing.T) {
	a := "Patients must not take more than 4 tablets in 24 hours (WHO, 2021)."
	cases := map[string]string{
		"negation": "Patients must take more than 4 tablets in 24 hours (WHO, 2021).",
		"number":   "Patients must not take more than 40 tablets in 24 hours (WHO, 2021).",
		"modal":    "Patients may not take more than 4 tablets in 24 hours (WHO, 2021).",
		"citation": "Patients must not take more than 4 tablets in 24 hours.",
	}
	for name, b := range cases {
		r := Compare(a, b)
		if r.Risk == "low" || len(r.Warnings) == 0 {
			t.Errorf("%s not detected: %+v", name, r)
		}
	}
}

func TestPlainRewriteLowRisk(t *testing.T) {
	a := "In order to apply, individuals must submit the form prior to 30 June."
	b := "To apply, people must submit the form before 30 June."
	r := Compare(a, b)
	if r.Risk == "high" {
		t.Fatalf("plain rewrite flagged high: %+v", r)
	}
}
