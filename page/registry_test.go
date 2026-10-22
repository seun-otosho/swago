package page

import "testing"

type testArticle struct {
	Author string `json:"author"`
}

func (testArticle) PageType() string { return "article" }

func TestRegisterNewDecode(t *testing.T) {
	Register[testArticle]()

	p, ok := New("article")
	if !ok {
		t.Fatal("article not registered")
	}
	if _, ok := p.(*testArticle); !ok {
		t.Fatalf("got %T, want *testArticle", p)
	}

	decoded, err := Decode("article", []byte(`{"author":"ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	a, ok := decoded.(*testArticle)
	if !ok || a.Author != "ada" {
		t.Fatalf("got %+v", decoded)
	}

	if _, err := Decode("article", nil); err != nil {
		t.Fatalf("empty data should decode to a zero value: %v", err)
	}

	if _, ok := New("unknown"); ok {
		t.Fatal("unknown type should not be registered")
	}
	if _, err := Decode("unknown", nil); err == nil {
		t.Fatal("decoding an unregistered type should error")
	}
}
