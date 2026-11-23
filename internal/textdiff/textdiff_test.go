package textdiff

import (
	"strconv"
	"strings"
	"testing"
)

func TestUnifiedDiff_SeparateHunks(t *testing.T) {
	var a, b []string
	for i := 1; i <= 20; i++ {
		a = append(a, "l"+strconv.Itoa(i))
	}
	b = append(b, a...)
	b[1] = "X"
	b[17] = "Y"
	got := UnifiedDiff(strings.Join(a, "\n")+"\n", strings.Join(b, "\n")+"\n", "f")
	if n := strings.Count(got, "@@ -"); n != 2 {
		t.Errorf("want 2 hunks, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "@@ -1,5 +1,5 @@\n l1\n-l2\n+X\n") || !strings.Contains(got, "@@ -15,6 +15,6 @@\n") {
		t.Errorf("hunks:\n%s", got)
	}
}
