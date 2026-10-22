package tree

import "testing"

func TestPaths(t *testing.T) {
	eq := func(got, want string) {
		t.Helper()
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	eq(Root(1), "0001")
	eq(Step(36), "0010")
	eq(Step(maxStep), "ZZZZ")
	eq(Parent("00010002"), "0001")
	eq(Parent("0001"), "")
	eq(FirstChild("0001"), "00010001")
	eq(Rebase("000100020001", "00010002", "0005"), "00050001")
	if Depth("000100020001") != 3 {
		t.Fatal("depth")
	}
	n, err := NextSibling("0001000Z")
	if err != nil {
		t.Fatal(err)
	}
	eq(n, "00010010")
	if _, err := NextSibling("ZZZZ"); err != ErrFull {
		t.Fatalf("want ErrFull, got %v", err)
	}
	if _, err := NextSibling("00-1"); err == nil {
		t.Fatal("want error for bad step")
	}
}
