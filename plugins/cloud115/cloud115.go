package cloud115

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/msterzhang/onelist/config"
)

const (
	PRO_API       = "https://proapi.115.com"
	PASSPORT_API  = "https://passportapi.115.com"
	WEB_API       = "https://webapi.115.com"
	SHARE_API     = "https://115cdn.com/webapi/share/snap"
	QRCODE_API    = "https://qrcodeapi.115.com"
	UA_CLOUD115   = "Mozilla/5.0 (115Tool/5.4)"
)

type bdmvCacheEntry struct {
	data      map[string]string
	expiresAt time.Time
}

var (
	bdmvCache   = make(map[string]bdmvCacheEntry)
	bdmvCacheMu sync.RWMutex
)

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

func testTokenValid(accessToken string) bool {
	api := fmt.Sprintf("%s/open/ufile/files?cid=0&limit=1", PRO_API)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode != 401
}

func refreshAccessToken(refreshToken string) (string, string, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	req, err := http.NewRequest("POST", PASSPORT_API+"/open/refreshToken", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var data Cloud115RspToken
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", "", err
	}
	if data.State {
		return data.Data.AccessToken, data.Data.RefreshToken, nil
	}
	return "", "", errors.New(data.Error)
}

func ensureValidToken() error {
	if !testTokenValid(config.Cloud115Token) {
		newAt, newRt, err := refreshAccessToken(config.Cloud115RefreshToken)
		if err != nil {
			return err
		}
		config.Cloud115Token = newAt
		config.Cloud115RefreshToken = newRt
		cfg := config.GetConfig()
		cfg.Cloud115Token = newAt
		cfg.Cloud115RefreshToken = newRt
		config.SaveConfig(cfg)
	}
	return nil
}

func getFilesByCid(accessToken string, cid string, offset int) ([]Cloud115FileEntry, int, error) {
	api := fmt.Sprintf("%s/open/ufile/files?cid=%s&limit=1150&offset=%d&asc=1&o=file_name&format=json", PRO_API, cid, offset)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	var data Cloud115RspFiles
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, 0, err
	}
	if !data.State {
		return nil, 0, errors.New(data.Error)
	}
	return data.Data, data.Count, nil
}

func getAllEntriesByCid(accessToken string, cid string) ([]Cloud115FileEntry, error) {
	var allEntries []Cloud115FileEntry
	offset := 0
	for {
		entries, _, err := getFilesByCid(accessToken, cid, offset)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		allEntries = append(allEntries, entries...)
		offset += len(entries)
		if len(entries) < 1150 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return allEntries, nil
}

func filterVideoEntry(entry Cloud115FileEntry) bool {
	if entry.Fc == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(entry.N))
	if ext == "" {
		return false
	}
	return strings.Contains(config.VideoTypes, ext)
}

func isBDMVFolder(entry Cloud115FileEntry) bool {
	return entry.Fc == 0 && strings.ToUpper(entry.N) == "BDMV"
}

func skipRecurseDir(name string) bool {
	upper := strings.ToUpper(name)
	skipDirs := []string{"CERTIFICATE"}
	for _, d := range skipDirs {
		if upper == d {
			return true
		}
	}
	return false
}

func listFilesRecursive(accessToken string, cid string, fileList []string) ([]string, error) {
	entries, err := getAllEntriesByCid(accessToken, cid)
	if err != nil {
		return fileList, err
	}
	for _, entry := range entries {
		if entry.Fc == 0 {
			if isBDMVFolder(entry) {
				fileList = append(fileList, "bdmv:"+cid)
				continue
			}
			if skipRecurseDir(entry.N) {
				continue
			}
			fileList, err = listFilesRecursive(accessToken, entry.Cid, fileList)
			if err != nil {
				return fileList, err
			}
		} else {
			if filterVideoEntry(entry) {
				fileList = append(fileList, entry.Pc)
			}
		}
	}
	return fileList, nil
}

func GetCloud115FilesPath(cid string) ([]string, error) {
	if cid == "" {
		cid = "0"
	}
	err := ensureValidToken()
	if err != nil {
		return nil, err
	}
	fileList := []string{}
	fileList, err = listFilesRecursive(config.Cloud115Token, cid, fileList)
	if err != nil {
		return nil, err
	}
	for _, f := range fileList {
		if strings.HasPrefix(f, "bdmv:") {
			bdmvCid := strings.TrimPrefix(f, "bdmv:")
			go getCachedBDMVTree(bdmvCid)
		}
	}
	return fileList, nil
}

func Cloud115RenameFile(fid string, newName string) error {
	err := ensureValidToken()
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("fid", fid)
	form.Set("file_name", newName)
	api := fmt.Sprintf("%s/files/rename", WEB_API)
	req, err := http.NewRequest("POST", api, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if config.Cloud115Cookie != "" {
		req.Header.Set("Cookie", config.Cloud115Cookie)
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var data Cloud115RspRename
	err = json.Unmarshal(body, &data)
	if err != nil {
		return err
	}
	if data.State {
		return nil
	}
	return errors.New(data.Error)
}

func Cloud115GetDownURL(pickCode string) (string, error) {
	err := ensureValidToken()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("pick_code", pickCode)
	api := fmt.Sprintf("%s/open/ufile/downurl", PRO_API)
	req, err := http.NewRequest("POST", api, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Authorization", "Bearer "+config.Cloud115Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data Cloud115RspDownURL
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}
	if data.State {
		for _, v := range data.Data {
			return v.URL.URL, nil
		}
	}
	return "", errors.New(data.Message)
}

func Cloud115GetVideoPreview(pickCode string) (Cloud115OpenVideo, error) {
	err := ensureValidToken()
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	form := url.Values{}
	form.Set("pick_code", pickCode)
	api := fmt.Sprintf("%s/open/video/video_preview", PRO_API)
	req, err := http.NewRequest("POST", api, strings.NewReader(form.Encode()))
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Authorization", "Bearer "+config.Cloud115Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	var data Cloud115RspVideoPreview
	err = json.Unmarshal(body, &data)
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	result := Cloud115OpenVideo{
		Code:    data.Code,
		Message: data.Message,
	}
	result.Data.DriveID = data.Data.DriveID
	result.Data.FileID = data.Data.FileID
	result.Data.VideoPreviewPlayInfo = data.Data.VideoPreviewPlayInfo
	if data.Code == 200 {
		return result, nil
	}
	return Cloud115OpenVideo{}, errors.New(data.Message)
}

func Cloud115GetQRCode() (string, string, int64, string, error) {
	api := fmt.Sprintf("%s/api/1.0/web/1.0/token/", QRCODE_API)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return "", "", 0, "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", "", 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, "", err
	}
	var data Cloud115RspQRCode
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", "", 0, "", err
	}
	if data.State {
		return data.Data.UID, data.Data.Qrcode, data.Data.Time, data.Data.Sign, nil
	}
	return "", "", 0, "", errors.New(data.Error)
}

func Cloud115CheckQRStatus(uid string, signTime int64, sign string) (int, error) {
	api := fmt.Sprintf("%s/get/status/?uid=%s&time=%d&sign=%s", QRCODE_API, uid, signTime, sign)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	resp, err := httpClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var data Cloud115RspQRStatus
	err = json.Unmarshal(body, &data)
	if err != nil {
		return 0, err
	}
	if data.State {
		return data.Data.Status, nil
	}
	return 0, nil
}

func Cloud115QRLogin(uid string) (string, error) {
	form := url.Values{}
	form.Set("app", "wechatmini")
	form.Set("account", uid)
	api := "https://passportapi.115.com/app/1.0/wechatmini/1.0/login/qrcode/"
	req, err := http.NewRequest("POST", api, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data Cloud115RspQRLogin
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}
	if data.State {
		var pairs []string
		for k, v := range data.Data.Cookie {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		return strings.Join(pairs, "; "), nil
	}
	return "", errors.New(data.Error)
}

func Cloud115ListBDMVFiles(cid string) ([]Cloud115FileEntry, error) {
	err := ensureValidToken()
	if err != nil {
		return nil, err
	}
	return getAllEntriesByCid(config.Cloud115Token, cid)
}

func Cloud115FindFileInBDMV(rootCid string, filePath string) (string, error) {
	tree, err := getCachedBDMVTree(rootCid)
	if err != nil {
		return "", err
	}
	cleanPath := strings.Trim(filePath, "/")
	pickCode, ok := tree[cleanPath]
	if !ok {
		return "", errors.New("file not found in BDMV: " + filePath)
	}
	return pickCode, nil
}

func getCachedBDMVTree(rootCid string) (map[string]string, error) {
	bdmvCacheMu.RLock()
	entry, exists := bdmvCache[rootCid]
	bdmvCacheMu.RUnlock()
	if exists && time.Now().Before(entry.expiresAt) {
		return entry.data, nil
	}

	err := ensureValidToken()
	if err != nil {
		return nil, err
	}

	tree := make(map[string]string)
	err = buildBDMVTree(config.Cloud115Token, rootCid, "", tree)
	if err != nil {
		return nil, err
	}

	bdmvCacheMu.Lock()
	bdmvCache[rootCid] = bdmvCacheEntry{
		data:      tree,
		expiresAt: time.Now().Add(30 * time.Minute),
	}
	bdmvCacheMu.Unlock()

	return tree, nil
}

func buildBDMVTree(accessToken string, cid string, prefix string, result map[string]string) error {
	entries, _, err := getFilesByCid(accessToken, cid, 0)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childPath := entry.N
		if prefix != "" {
			childPath = prefix + "/" + entry.N
		}
		if entry.Fc == 0 {
			err = buildBDMVTree(accessToken, entry.Cid, childPath, result)
			if err != nil {
				return err
			}
		} else if entry.Pc != "" {
			result[childPath] = entry.Pc
		}
	}
	return nil
}

func Cloud115GetBDMVDownURL(pickCode string) (string, error) {
	err := ensureValidToken()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("pick_code", pickCode)
	api := fmt.Sprintf("%s/open/ufile/downurl", PRO_API)
	req, err := http.NewRequest("POST", api, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Authorization", "Bearer "+config.Cloud115Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data Cloud115RspDownURL
	err = json.Unmarshal(body, &data)
	if err != nil {
		return "", err
	}
	if data.State {
		for _, v := range data.Data {
			return v.URL.URL, nil
		}
	}
	return "", errors.New(data.Message)
}

func ParseShareURL(shareURL string) (shareCode string, receiveCode string, err error) {
	re := regexp.MustCompile(`/s/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(shareURL)
	if len(matches) < 2 {
		return "", "", errors.New("无法解析分享链接")
	}
	shareCode = matches[1]
	parsed, err := url.Parse(shareURL)
	if err != nil {
		return shareCode, "", nil
	}
	receiveCode = parsed.Query().Get("password")
	if receiveCode == "" {
		receiveCode = parsed.Query().Get("pwd")
	}
	return shareCode, receiveCode, nil
}

func Fetch115ShareEntries(shareCode string, receiveCode string, cid string, offset int) ([]Cloud115FileEntry, int, error) {
	api := fmt.Sprintf("%s?share_code=%s&receive_code=%s&cid=%s&offset=%d&limit=1150&asc=1&o=file_name&format=json",
		SHARE_API, shareCode, receiveCode, cid, offset)
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	var data Cloud115RspShareFiles
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, 0, err
	}
	if !data.State {
		return nil, 0, errors.New(data.Error)
	}
	return data.Data.List, data.Data.Count, nil
}

func Get115ShareTree(shareURL string) ([]Cloud115FileEntry, error) {
	shareCode, receiveCode, err := ParseShareURL(shareURL)
	if err != nil {
		return nil, err
	}
	var allEntries []Cloud115FileEntry
	offset := 0
	for {
		entries, _, err := Fetch115ShareEntries(shareCode, receiveCode, "0", offset)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		allEntries = append(allEntries, entries...)
		offset += len(entries)
		if len(entries) < 1150 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return allEntries, nil
}

func Get115ShareSubEntries(shareURL string, cid string) ([]Cloud115FileEntry, error) {
	shareCode, receiveCode, err := ParseShareURL(shareURL)
	if err != nil {
		return nil, err
	}
	var allEntries []Cloud115FileEntry
	offset := 0
	for {
		entries, _, err := Fetch115ShareEntries(shareCode, receiveCode, cid, offset)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		allEntries = append(allEntries, entries...)
		offset += len(entries)
		if len(entries) < 1150 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return allEntries, nil
}

func Transfer115ShareFile(shareURL string, fid string) (string, string, error) {
	shareCode, receiveCode, err := ParseShareURL(shareURL)
	if err != nil {
		return "", "", err
	}
	headers := map[string]string{
		"User-Agent": UA_CLOUD115,
		"Referer":    "https://115cdn.com/s/" + shareCode,
	}
	if config.Cloud115Cookie != "" {
		headers["Cookie"] = config.Cloud115Cookie
	}
	form := url.Values{}
	form.Set("share_code", shareCode)
	form.Set("receive_code", receiveCode)
	form.Set("file_id", fid)

	req, err := http.NewRequest("POST", "https://115cdn.com/webapi/share/receive", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", UA_CLOUD115)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if config.Cloud115Cookie != "" {
		req.Header.Set("Cookie", config.Cloud115Cookie)
	}
	req.Header.Set("Referer", "https://115cdn.com/s/"+shareCode)
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	if state, ok := result["state"].(bool); !ok || !state {
		msg := ""
		if m, ok := result["msg"].(string); ok {
			if !strings.Contains(m, "无需重复接收") {
				return "", "", errors.New(m)
			}
		} else {
			return "", "", errors.New(msg)
		}
	}
	time.Sleep(2 * time.Second)

	rootReq, err := http.NewRequest("GET", WEB_API+"/files?cid=0", nil)
	if err != nil {
		return "", "", err
	}
	rootReq.Header.Set("User-Agent", UA_CLOUD115)
	if config.Cloud115Cookie != "" {
		rootReq.Header.Set("Cookie", config.Cloud115Cookie)
	}
	rootResp, err := httpClient().Do(rootReq)
	if err != nil {
		return "", "", err
	}
	defer rootResp.Body.Close()
	rootBody, _ := io.ReadAll(rootResp.Body)
	var rootData Cloud115RspFiles
	json.Unmarshal(rootBody, &rootData)
	var receiveCid string
	for _, item := range rootData.Data {
		if item.N == "最近接收" && item.Fc == 0 {
			receiveCid = item.Cid
			break
		}
	}
	if receiveCid == "" {
		return "", "", errors.New("未找到最近接收目录")
	}

	params := url.Values{}
	params.Set("cid", receiveCid)
	params.Set("o", "user_ptime")
	params.Set("asc", "0")
	params.Set("limit", "30")
	params.Set("format", "json")
	filesReq, err := http.NewRequest("GET", WEB_API+"/files?"+params.Encode(), nil)
	if err != nil {
		return "", "", err
	}
	filesReq.Header.Set("User-Agent", UA_CLOUD115)
	if config.Cloud115Cookie != "" {
		filesReq.Header.Set("Cookie", config.Cloud115Cookie)
	}
	filesResp, err := httpClient().Do(filesReq)
	if err != nil {
		return "", "", err
	}
	defer filesResp.Body.Close()
	filesBody, _ := io.ReadAll(filesResp.Body)
	var filesData Cloud115RspFiles
	json.Unmarshal(filesBody, &filesData)
	if !filesData.State {
		return "", "", errors.New("获取转存文件列表失败")
	}

	for _, f := range filesData.Data {
		if f.Fid != "" {
			return f.Fid, f.Pc, nil
		}
	}
	return "", "", errors.New("转存成功但未找到文件")
}

func TransferAndScrapeShare(shareURL string) ([]string, error) {
	entries, err := Get115ShareTree(shareURL)
	if err != nil {
		return nil, err
	}
	var fileList []string
	for _, entry := range entries {
		if entry.Fc == 0 {
			subEntries, err := Get115ShareSubEntries(shareURL, entry.Cid)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if sub.Fc == 0 && strings.ToUpper(sub.N) == "BDMV" {
					_, pc, err := Transfer115ShareFile(shareURL, entry.Fid)
					if err != nil {
						continue
					}
					bdmvCid := ""
					if pc != "" {
						fileList = append(fileList, "bdmv:"+bdmvCid)
					}
					continue
				}
				if filterVideoEntry(sub) && sub.Fc != 0 {
					_, pc, err := Transfer115ShareFile(shareURL, sub.Fid)
					if err != nil {
						continue
					}
					if pc != "" {
						fileList = append(fileList, pc)
					}
				}
			}
		} else {
			if filterVideoEntry(entry) {
				_, pc, err := Transfer115ShareFile(shareURL, entry.Fid)
				if err != nil {
					continue
				}
				if pc != "" {
					fileList = append(fileList, pc)
				}
			}
		}
	}
	return fileList, nil
}
