package creatures

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveImages(t *testing.T) {
	// Trimmed real response for Dragon, Hot Dog (redirects to Dog) and a
	// creature with no image file.
	const body = `{"batchcomplete":"","query":{
		"normalized":[{"from":"File:Hot_Dog.gif","to":"File:Hot Dog.gif"}],
		"redirects":[{"from":"File:Hot Dog.gif","to":"File:Dog.gif"}],
		"pages":{
			"1317":{"pageid":1317,"ns":6,"title":"File:Dragon.gif","imageinfo":[{"url":"https://cdn/e/e0/Dragon.gif"}]},
			"828":{"pageid":828,"ns":6,"title":"File:Dog.gif","imageinfo":[{"url":"https://cdn/9/99/Dog.gif"}]},
			"-1":{"ns":6,"title":"File:Nobody.gif","missing":""}}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	got, err := resolveImages(context.Background(), srv.Client(), srv.URL, []string{"Dragon", "Hot_Dog", "Nobody"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Dragon": "https://cdn/e/e0/Dragon.gif", "Hot_Dog": "https://cdn/9/99/Dog.gif"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}
