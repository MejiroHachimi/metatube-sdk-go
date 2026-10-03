package avleague

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Entirely synthetic profile; no upstream body is checked into the fixture.
func TestFixtureActorProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><div id="pan"><span>Fixture Actor</span></div>
<div id="j-prof"><span>別名: Fixture Alias</span></div>
<div id="contents"><div class="i-pic-box"><div><img src="/avatar.jpg"></div></div><table><tbody>
<tr><th>生年月日</th><td>1990年1月2日</td></tr>
<tr><th>身長</th><td>165cm</td></tr>
<tr><th>血液型</th><td>AB型</td></tr>
<tr><th>3サイズ</th><td>B:88（D） / W:60 / H:90</td></tr>
<tr><th>デビュー</th><td>2012年3月4日</td></tr>
</tbody></table></div></html>`))
	}))
	defer server.Close()
	info, err := New().GetActorInfoByURL(server.URL + "/actress/123.html")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsValid() || info.ID != "123" || info.Name != "Fixture Actor" {
		t.Fatal("profile identity missing")
	}
	if got := time.Time(info.Birthday).Format("2006-01-02"); got != "1990-01-02" {
		t.Fatalf("birthday: %s", got)
	}
	if info.Height != 165 || info.BloodType != "AB" || info.CupSize != "D" || info.Measurements != "B:88 / W:60 / H:90" {
		t.Fatal("profile field mapping failed")
	}
	if len(info.Aliases) != 1 || info.Aliases[0] != "Fixture Alias" || len(info.Images) != 1 || info.Images[0] != server.URL+"/avatar.jpg" {
		t.Fatal("aliases/images missing")
	}
}
