package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 影库
type Gallery struct {
	Id                  uint      `json:"id" gorm:"primaryKey"`         //ID
	Title               string    `json:"title" gorm:"not null;unique"` //标题
	GalleryType         string    `json:"gallery_type"`                 //影库类型，电影或者电视
	IsTv                bool      `json:"is_tv"`                        //影库类型，是否是电视
	GalleryUid          string    `json:"gallery_uid"`                  //唯一uid
	Image               string    `json:"image"`                        //图片
	IsCloud115          bool      `json:"is_cloud115"`                  //是否是115云盘直连
	Cloud115Token       string    `json:"cloud115_token"`               //115 access_token
	Cloud115RefreshToken string   `json:"cloud115_refresh_token"`       //115 refresh_token
	Cloud115Cookie      string    `json:"cloud115_cookie"`              //115 cookie
	Works               []Work    `json:"works"`                        //添加的目录列表
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (g *Gallery) BeforeCreate(tx *gorm.DB) (err error) {
	g.GalleryUid = uuid.New().String()
	g.CreatedAt = time.Now()
	g.UpdatedAt = time.Now()
	return
}

func (g *Gallery) BeforeUpdate(tx *gorm.DB) (err error) {
	g.UpdatedAt = time.Now()
	return
}
