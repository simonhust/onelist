package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/msterzhang/onelist/api/utils/cache"
	"github.com/msterzhang/onelist/config"
	"github.com/msterzhang/onelist/plugins/cloud115"
)

func Get115QRCode(c *gin.Context) {
	galleryUid := c.Query("gallery_uid")
	uid, qrcode, signTime, sign, err := cloud115.Cloud115GetQRCode()
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error(), "data": ""})
		return
	}
	cache.NewCache().Set("qrcode:"+uid, map[string]interface{}{
		"sign_time": signTime,
		"sign":      sign,
	}, 5*time.Minute)
	c.JSON(200, gin.H{"code": 200, "msg": "获取二维码成功", "data": gin.H{
		"uid":         uid,
		"qrcode":      qrcode,
		"gallery_uid": galleryUid,
	}})
}

func Get115QRCodeStatus(c *gin.Context) {
	uid := c.Query("uid")
	cached, found := cache.NewCache().Get("qrcode:" + uid)
	if !found {
		c.JSON(200, gin.H{"code": 201, "msg": "二维码已过期", "data": gin.H{"status": -1}})
		return
	}
	info := cached.(map[string]interface{})
	signTime := info["sign_time"].(int64)
	sign := info["sign"].(string)

	status, err := cloud115.Cloud115CheckQRStatus(uid, signTime, sign)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error(), "data": gin.H{"status": 0}})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "查询成功", "data": gin.H{"status": status}})
}

func Post115QRCodeLogin(c *gin.Context) {
	uid := c.PostForm("uid")
	_ = c.PostForm("gallery_uid")

	cookie, err := cloud115.Cloud115QRLogin(uid)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error(), "data": ""})
		return
	}

	if cookie != "" {
		cfg := config.GetConfig()
		cfg.Cloud115Cookie = cookie
		config.SaveConfig(cfg)
	}

	cache.NewCache().Delete("qrcode:" + uid)
	c.JSON(200, gin.H{"code": 200, "msg": "登录成功", "data": gin.H{"cookie": cookie}})
}

func Proxy115File(c *gin.Context) {
	_ = c.Param("gallery_uid")
	pickCode := c.Param("pick_code")
	pickCode = strings.TrimPrefix(pickCode, "/")

	if strings.HasPrefix(pickCode, "share:") {
		parts := strings.SplitN(pickCode, ":", 3)
		if len(parts) != 3 {
			c.String(http.StatusBadRequest, "无效的分享文件标识")
			return
		}
		sharePC := parts[1]
		shareURL := parts[2]
		pickCode, err := cloud115.TransferSingleShareFile(shareURL, sharePC)
		if err != nil {
			c.String(http.StatusInternalServerError, "转存失败: %s", err.Error())
			return
		}
		dlURL, err := cloud115.Cloud115GetDownURL(pickCode)
		if err != nil {
			c.String(http.StatusInternalServerError, "获取下载链接失败: %s", err.Error())
			return
		}
		c.Redirect(http.StatusFound, dlURL)
		return
	}

	dlURL, err := cloud115.Cloud115GetDownURL(pickCode)
	if err != nil {
		c.String(http.StatusInternalServerError, "获取下载链接失败: %s", err.Error())
		return
	}
	c.Redirect(http.StatusFound, dlURL)
}

func Get115ShareTree(c *gin.Context) {
	shareURL := c.Query("url")
	if shareURL == "" {
		c.JSON(200, gin.H{"code": 201, "msg": "缺少分享链接"})
		return
	}
	entries, err := cloud115.Get115ShareTree(shareURL)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "success", "data": entries})
}

func Post115ShareTransfer(c *gin.Context) {
	shareURL := c.PostForm("url")
	if shareURL == "" {
		c.JSON(200, gin.H{"code": 201, "msg": "缺少分享链接"})
		return
	}
	files, err := cloud115.TransferAndScrapeShare(shareURL)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 200, "msg": "转存成功", "data": gin.H{"files": files, "count": len(files)}})
}

func Proxy115BDMV(c *gin.Context) {
	_ = c.Param("gallery_uid")
	cid := c.Param("cid")
	filepath := c.Param("filepath")

	if filepath == "" || filepath == "/" {
		entries, err := cloud115.Cloud115ListBDMVFiles(cid)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": err.Error(), "data": ""})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "success", "data": entries})
		return
	}

	pickCode, err := cloud115.Cloud115FindFileInBDMV(cid, filepath)
	if err != nil {
		c.String(http.StatusNotFound, "文件不存在: %s", err.Error())
		return
	}

	dlURL, err := cloud115.Cloud115GetBDMVDownURL(pickCode)
	if err != nil {
		c.String(http.StatusInternalServerError, "获取下载链接失败: %s", err.Error())
		return
	}
	c.Redirect(http.StatusFound, dlURL)
}
