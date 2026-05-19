package models

type Config struct {
	Title                string `json:"title"`
	DownLoadImage        string `json:"download_image"`
	ImgUrl               string `json:"img_url"`
	KeyDb                string `json:"key_db"`
	FaviconicoUrl        string `json:"faviconico_url"`
	VideoTypes           string `json:"video_types"`
	Cloud115Token        string `json:"cloud115_token"`
	Cloud115RefreshToken string `json:"cloud115_refresh_token"`
	Cloud115Cookie       string `json:"cloud115_cookie"`
}
