package quark

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/go-resty/resty/v2"
)

// mobilePlayURL only carries the signed-in device parameters to the playback
// endpoint. File-specific parameters from the captured request must not be reused.
func mobilePlayURL(raw string) (*url.URL, error) {
	if strings.ContainsAny(raw, "\r\n\x00") {
		return nil, errors.New("invalid Quark mobile request URL")
	}
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > 32*1024 {
		return nil, errors.New("invalid Quark mobile request URL")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "drive-m.quark.cn" || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/1/clouddrive/") {
		return nil, errors.New("Quark mobile request URL must use drive-m.quark.cn/1/clouddrive")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, errors.New("invalid Quark mobile request parameters")
	}
	for _, key := range []string{"kps", "sign", "vcode", "ut"} {
		if len(query[key]) != 1 || query.Get(key) == "" {
			return nil, fmt.Errorf("Quark mobile request URL is missing %s", key)
		}
	}
	for _, key := range []string{"fid", "fids", "pdir_fid", "share_id", "share_token", "share_fid_token", "resolutions", "supports", "_page", "_size", "_fetch_total", "fetch_all_file", "fetch_risk_file_name", "_sort", "_t"} {
		query.Del(key)
	}
	if query.Get("pr") == "" {
		query.Set("pr", "ucpro")
	}
	if query.Get("fr") == "" {
		query.Set("fr", "android")
	}
	u.Path = "/1/clouddrive/file/v2/play/project"
	u.RawPath = ""
	u.RawQuery = query.Encode()
	return u, nil
}

func (d *QuarkOrUC) getMobileTranscodingLink(body base.Json) (TranscodingResp, error) {
	var resp TranscodingResp
	if d.conf.api != "https://drive.quark.cn/1/clouddrive" {
		return resp, errors.New("mobile playback is only supported for Quark")
	}
	u, err := mobilePlayURL(d.MobileRequestURL)
	if err != nil {
		return resp, err
	}
	var apiErr Resp
	// Keep the shared transport, but never forward credentials through a redirect.
	httpClient := *base.RestyClient.GetClient()
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client := resty.NewWithClient(&httpClient)
	res, err := client.R().
		SetHeaders(map[string]string{
			"Cookie":     d.Cookie,
			"Accept":     "application/json, text/plain, */*",
			"Referer":    d.conf.referer,
			"User-Agent": d.conf.ua,
		}).
		SetBody(body).
		SetResult(&resp).
		SetError(&apiErr).
		Execute(http.MethodPost, u.String())
	if err != nil {
		// Resty errors can contain the URL and its authentication parameters.
		return resp, errors.New("Quark mobile playback request failed")
	}
	if res.StatusCode() != http.StatusOK || apiErr.Code != 0 || resp.Code != 0 || resp.Status >= http.StatusBadRequest {
		if res.StatusCode() == http.StatusUnauthorized || res.StatusCode() == http.StatusForbidden {
			return resp, errors.New("Quark mobile playback credentials are invalid or expired")
		}
		if strings.Contains(strings.ToLower(apiErr.Message), "plf_invalid") {
			return resp, errors.New("Quark rejected the mobile playback credentials (plf_invalid)")
		}
		code := apiErr.Code
		if code == 0 {
			code = resp.Code
		}
		return resp, fmt.Errorf("Quark mobile playback request failed (HTTP %d, code %d)", res.StatusCode(), code)
	}
	return resp, nil
}
