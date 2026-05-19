package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/msterzhang/onelist/api/database"
	"github.com/msterzhang/onelist/api/models"
	"github.com/msterzhang/onelist/api/utils/cache"
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
	galleryUid := c.PostForm("gallery_uid")

	cookie, err := cloud115.Cloud115QRLogin(uid)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": err.Error(), "data": ""})
		return
	}

	if galleryUid != "" {
		db := database.NewDb()
		gallery := models.Gallery{}
		err := db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).First(&gallery).Error
		if err == nil {
			gallery.Cloud115Cookie = cookie
			db.Model(&models.Gallery{}).Where("gallery_uid = ?", galleryUid).Select("*").Updates(&gallery)
		}
	}

	cache.NewCache().Delete("qrcode:" + uid)
	c.JSON(200, gin.H{"code": 200, "msg": "登录成功", "data": gin.H{"cookie": cookie}})
}
