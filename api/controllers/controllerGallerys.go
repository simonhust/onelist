package controllers

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/msterzhang/onelist/api/database"
	"github.com/msterzhang/onelist/api/models"
	"github.com/msterzhang/onelist/api/repository"
	"github.com/msterzhang/onelist/api/repository/crud"
	"github.com/msterzhang/onelist/api/utils/dir"
	"github.com/msterzhang/onelist/plugins/cloud115"

	"github.com/gin-gonic/gin"
)

func parseShareAndStartScrape(gallery models.Gallery) (int, error) {
	var path string
	if gallery.ShareURL != "" {
		path = gallery.ShareURL
		gallery.IsCloud115 = true
	}
	if path == "" {
		return 0, nil
	}
	var files []string
	var err error
	if gallery.IsCloud115 {
		files, err = cloud115.GetCloud115FilesPath(path)
	} else {
		files = dir.GetFilesByPath(path)
	}
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, errors.New("分享链接中未找到视频文件")
	}
	db := database.NewDb()
	work := models.Work{
		GalleryUid: gallery.GalleryUid,
		Path:       path,
		FileNumber: len(files),
	}
	err = db.Model(&models.Work{}).Create(&work).Error
	if err != nil {
		return 0, err
	}
	go RunWork(files, work, gallery)
	return len(files), nil
}

func CreateGallery(c *gin.Context) {
	gallery := models.Gallery{}
	err := c.ShouldBind(&gallery)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": "创建失败,表单解析出错!", "data": gallery})
		return
	}
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallery, err := galleryRepository.Save(gallery)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "创建失败!", "data": gallery})
			return
		}
		count, err := parseShareAndStartScrape(gallery)
		if err != nil {
			c.JSON(200, gin.H{"code": 200, "msg": "创建成功，刮削失败: " + err.Error(), "data": gallery, "file_count": 0})
			return
		}
		if count > 0 {
			c.JSON(200, gin.H{"code": 200, "msg": fmt.Sprintf("创建成功，发现 %d 个视频文件，正在刮削...", count), "data": gallery, "file_count": count})
		} else {
			c.JSON(200, gin.H{"code": 200, "msg": "创建成功!", "data": gallery, "file_count": 0})
		}
	}(repo)
}

func DeleteGalleryById(c *gin.Context) {
	id := c.Query("id")
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallery, err := galleryRepository.DeleteByID(id)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallery})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "删除资源成功!", "data": gallery})
	}(repo)
}

func UpdateGalleryById(c *gin.Context) {
	id := c.Query("id")
	gallery := models.Gallery{}
	err := c.ShouldBind(&gallery)
	if err != nil {
		c.JSON(200, gin.H{"code": 201, "msg": "创建失败,表单解析出错!", "data": gallery})
		return
	}
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		_, err := galleryRepository.UpdateByID(id, gallery)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallery})
			return
		}
		gallery, _ = galleryRepository.FindByUID(gallery.GalleryUid)
		count, err := parseShareAndStartScrape(gallery)
		if err != nil {
			c.JSON(200, gin.H{"code": 200, "msg": "更新成功，刮削失败: " + err.Error(), "data": gallery, "file_count": 0})
			return
		}
		if count > 0 {
			c.JSON(200, gin.H{"code": 200, "msg": fmt.Sprintf("更新成功，发现 %d 个视频文件，正在刮削...", count), "data": gallery, "file_count": count})
		} else {
			c.JSON(200, gin.H{"code": 200, "msg": "更新资源成功!", "data": gallery, "file_count": 0})
		}
	}(repo)
}

func GetGalleryById(c *gin.Context) {
	id := c.Query("id")
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallery, err := galleryRepository.FindByID(id)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallery})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": gallery})
	}(repo)
}

func GetGalleryList(c *gin.Context) {
	page, errPage := strconv.Atoi(c.Query("page"))
	size, errSize := strconv.Atoi(c.Query("size"))
	if errPage != nil {
		page = 1
	}
	if errSize != nil {
		size = 8
	}
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallerys, num, err := galleryRepository.FindAll(page, size)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallerys, "num": num})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": gallerys, "num": num})
	}(repo)
}

func GetGalleryListAdmin(c *gin.Context) {
	page, errPage := strconv.Atoi(c.Query("page"))
	size, errSize := strconv.Atoi(c.Query("size"))
	if errPage != nil {
		page = 1
	}
	if errSize != nil {
		size = 8
	}
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallerys, num, err := galleryRepository.FindAllByAdmin(page, size)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallerys, "num": num})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": gallerys, "num": num})
	}(repo)
}


func GetGalleryHostByUid(c *gin.Context) {
	id := c.Query("id")
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallery, err := galleryRepository.FindByUID(id)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": "", "is_cloud115": false, "share_url": ""})
			return
		}
		if gallery.IsCloud115 {
			c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": "https://proapi.115.com", "is_cloud115": true, "share_url": gallery.ShareURL})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": "", "is_cloud115": false, "share_url": ""})
	}(repo)
}

func SearchGallery(c *gin.Context) {
	q := c.Query("q")
	if len(q) == 0 {
		c.JSON(200, gin.H{"code": 201, "msg": "参数错误!", "data": ""})
		return
	}
	page, errPage := strconv.Atoi(c.Query("page"))
	size, errSize := strconv.Atoi(c.Query("size"))
	if errPage != nil {
		page = 1
	}
	if errSize != nil {
		size = 8
	}
	db := database.NewDb()
	repo := crud.NewRepositoryGallerysCRUD(db)
	func(galleryRepository repository.GalleryRepository) {
		gallerys, num, err := galleryRepository.Search(q, page, size)
		if err != nil {
			c.JSON(200, gin.H{"code": 201, "msg": "没有查询到资源!", "data": gallerys, "num": num})
			return
		}
		c.JSON(200, gin.H{"code": 200, "msg": "查询资源成功!", "data": gallerys, "num": num})
	}(repo)
}
