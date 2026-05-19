package cloud115

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/msterzhang/onelist/api/database"
	"github.com/msterzhang/onelist/api/models"
	"github.com/msterzhang/onelist/config"
)

const (
	PRO_API       = "https://proapi.115.com"
	PASSPORT_API  = "https://passportapi.115.com"
	WEB_API       = "https://webapi.115.com"
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

func refreshToken(refreshToken string) (string, string, error) {
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

func ensureValidToken(gallery *models.Gallery) error {
	if !testTokenValid(gallery.Cloud115Token) {
		newAt, newRt, err := refreshToken(gallery.Cloud115RefreshToken)
		if err != nil {
			return err
		}
		gallery.Cloud115Token = newAt
		gallery.Cloud115RefreshToken = newRt
		db := database.NewDb()
		err = db.Model(&models.Gallery{}).Where("id = ?", gallery.Id).Select("*").Updates(gallery).Error
		if err != nil {
			return err
		}
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

func GetCloud115FilesPath(cid string, gallery models.Gallery) ([]string, error) {
	if cid == "" {
		cid = "0"
	}
	err := ensureValidToken(&gallery)
	if err != nil {
		return nil, err
	}
	fileList := []string{}
	fileList, err = listFilesRecursive(gallery.Cloud115Token, cid, fileList)
	if err != nil {
		return nil, err
	}
	for _, f := range fileList {
		if strings.HasPrefix(f, "bdmv:") {
			bdmvCid := strings.TrimPrefix(f, "bdmv:")
			go getCachedBDMVTree(gallery.GalleryUid, bdmvCid)
		}
	}
	return fileList, nil
}

func Cloud115RenameFile(fid string, newName string, galleryUid string) error {
	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return err
	}
	err = ensureValidToken(&gallery)
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
	if gallery.Cloud115Cookie != "" {
		req.Header.Set("Cookie", gallery.Cloud115Cookie)
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

func Cloud115GetDownURL(pickCode string, galleryUid string) (string, error) {
	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return "", err
	}
	err = ensureValidToken(&gallery)
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
	req.Header.Set("Authorization", "Bearer "+gallery.Cloud115Token)
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

func Cloud115GetVideoPreview(pickCode string, galleryUid string) (Cloud115OpenVideo, error) {
	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return Cloud115OpenVideo{}, err
	}
	err = ensureValidToken(&gallery)
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
	req.Header.Set("Authorization", "Bearer "+gallery.Cloud115Token)
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
	api := fmt.Sprintf("https://qrcodeapi.115.com/api/1.0/web/1.0/token/")
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
	api := fmt.Sprintf("https://qrcodeapi.115.com/get/status/?uid=%s&time=%d&sign=%s", uid, signTime, sign)
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

func Cloud115ListBDMVFiles(galleryUid string, cid string) ([]Cloud115FileEntry, error) {
	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return nil, err
	}
	err = ensureValidToken(&gallery)
	if err != nil {
		return nil, err
	}
	return getAllEntriesByCid(gallery.Cloud115Token, cid)
}

func Cloud115FindFileInBDMV(galleryUid string, rootCid string, filePath string) (string, error) {
	tree, err := getCachedBDMVTree(galleryUid, rootCid)
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

func getCachedBDMVTree(galleryUid string, rootCid string) (map[string]string, error) {
	bdmvCacheMu.RLock()
	entry, exists := bdmvCache[rootCid]
	bdmvCacheMu.RUnlock()
	if exists && time.Now().Before(entry.expiresAt) {
		return entry.data, nil
	}

	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return nil, err
	}
	err = ensureValidToken(&gallery)
	if err != nil {
		return nil, err
	}

	tree := make(map[string]string)
	err = buildBDMVTree(gallery.Cloud115Token, rootCid, "", tree)
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

func Cloud115GetBDMVDownURL(galleryUid string, pickCode string) (string, error) {
	gallery := models.Gallery{}
	db := database.NewDb()
	err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
	if err != nil {
		return "", err
	}
	err = ensureValidToken(&gallery)
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
	req.Header.Set("Authorization", "Bearer "+gallery.Cloud115Token)
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
