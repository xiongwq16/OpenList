package quark

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/go-resty/resty/v2"
)

type mobileRoundTripFunc func(*http.Request) (*http.Response, error)

func (f mobileRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMobilePlayURL(t *testing.T) {
	u, err := mobilePlayURL("https://drive-m.quark.cn/1/clouddrive/file/sort?pr=ucpro&fr=android&kps=k&sign=s&vcode=v&ut=u&pdir_fid=123&_page=2")
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/1/clouddrive/file/v2/play/project" {
		t.Fatalf("unexpected endpoint: %s", u.Path)
	}
	q := u.Query()
	for key, want := range map[string]string{"pr": "ucpro", "fr": "android", "kps": "k", "sign": "s", "vcode": "v", "ut": "u"} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if q.Has("pdir_fid") || q.Has("_page") {
		t.Fatal("captured operation parameters were retained")
	}
}

func TestMobilePlayURLRejectsUnsafeOrIncompleteInput(t *testing.T) {
	for _, raw := range []string{
		"http://drive-m.quark.cn/1/clouddrive/file/sort?kps=k&sign=s&vcode=v&ut=u",
		"https://drive-m.quark.cn.evil.example/1/clouddrive/file/sort?kps=k&sign=s&vcode=v&ut=u",
		"https://drive-m.quark.cn/other/file/sort?kps=k&sign=s&vcode=v&ut=u",
		"https://drive-m.quark.cn/1/clouddrive/file/sort?kps=k&sign=s&vcode=v",
		"https://drive-m.quark.cn/1/clouddrive/file/sort?kps=k&kps=other&sign=s&vcode=v&ut=u",
		"https://drive-m.quark.cn/1/clouddrive/file/sort?kps=k&sign=s&vcode=v&ut=u\n",
	} {
		if _, err := mobilePlayURL(raw); err == nil {
			t.Errorf("accepted unsafe or incomplete URL: %q", raw)
		}
	}
	if _, err := mobilePlayURL(strings.Repeat("x", 32*1024+1)); err == nil {
		t.Fatal("accepted oversized URL")
	}
}

func TestMobileTranscodingRequestUsesQuarkHostAndExistingCookie(t *testing.T) {
	previousClient := base.RestyClient
	defer func() { base.RestyClient = previousClient }()
	base.RestyClient = resty.New().SetTransport(mobileRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "drive-m.quark.cn" || req.URL.Path != "/1/clouddrive/file/v2/play/project" {
			t.Errorf("unexpected playback endpoint: %s", req.URL.Host+req.URL.Path)
		}
		if req.URL.Query().Get("kps") != "k" || req.URL.Query().Has("pdir_fid") {
			t.Error("mobile authentication or captured operation parameters were lost")
		}
		if req.Header.Get("Cookie") != "session=test" {
			t.Error("existing Quark cookie was not forwarded")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil || !strings.Contains(string(body), `"fid":"video-id"`) {
			t.Errorf("unexpected playback request body: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":200,"code":0,"data":{"video_list":[{"video_info":{"url":"https://video.example/stream"}}]}}`)),
			Request:    req,
		}, nil
	}))
	d := &QuarkOrUC{
		Addition: Addition{
			Cookie:           "session=test",
			MobileRequestURL: "https://drive-m.quark.cn/1/clouddrive/file/sort?fr=android&kps=k&sign=s&vcode=v&ut=u&pdir_fid=123",
		},
		conf: Conf{api: "https://drive.quark.cn/1/clouddrive", referer: "https://pan.quark.cn", ua: "test-agent"},
	}
	resp, err := d.getMobileTranscodingLink(base.Json{"fid": "video-id"})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Data.VideoList[0].VideoInfo.URL; got != "https://video.example/stream" {
		t.Fatalf("unexpected video URL: %s", got)
	}
}

func TestMobileTranscodingRejectsUC(t *testing.T) {
	d := &QuarkOrUC{conf: Conf{api: "https://pc-api.uc.cn/1/clouddrive"}}
	if _, err := d.getMobileTranscodingLink(base.Json{"fid": "video-id"}); err == nil {
		t.Fatal("UC credentials could be sent to the Quark mobile host")
	}
}

func TestMobileTranscodingDoesNotFollowRedirect(t *testing.T) {
	previousClient := base.RestyClient
	defer func() { base.RestyClient = previousClient }()
	requests := 0
	base.RestyClient = resty.New().SetTransport(mobileRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://other.example/steal"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	}))
	d := &QuarkOrUC{
		Addition: Addition{MobileRequestURL: "https://drive-m.quark.cn/1/clouddrive/file/sort?kps=k&sign=s&vcode=v&ut=u"},
		conf:     Conf{api: "https://drive.quark.cn/1/clouddrive"},
	}
	if _, err := d.getMobileTranscodingLink(base.Json{"fid": "video-id"}); err == nil {
		t.Fatal("redirect was accepted")
	}
	if requests != 1 {
		t.Fatalf("redirect was followed: %d requests", requests)
	}
}
