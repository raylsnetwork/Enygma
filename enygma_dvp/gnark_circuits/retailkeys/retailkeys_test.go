package retailkeys

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// make dvp-keygen copies retail's keys for the circuits the Makefile's
// DVP_RETAIL_KEYS names, and the export tools Require every Payment-family
// circuit's: a circuit missing from that list would stop the stack on a key
// make dvp-keygen never copies.
func TestMakefileCopiesEveryPaymentFamilyKey(t *testing.T) {
	mk, err := os.ReadFile("../../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^DVP_RETAIL_KEYS := (.+)$`).FindSubmatch(mk)
	if m == nil {
		t.Fatal("the Makefile has no DVP_RETAIL_KEYS := line")
	}
	copied := strings.Fields(string(m[1]))
	for _, c := range PaymentFamily {
		if !slices.Contains(copied, c) {
			t.Errorf("DVP_RETAIL_KEYS (%s) does not name %s, whose keys the export tools require to be retail's", m[1], c)
		}
	}
}
