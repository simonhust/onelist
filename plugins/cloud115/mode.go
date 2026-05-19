package cloud115

type Cloud115RspToken struct {
	State bool `json:"state"`
	Data  struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	} `json:"data"`
	Error string `json:"error"`
}

type Cloud115FileEntry struct {
	N   string `json:"n"`
	Fid string `json:"fid"`
	Cid string `json:"cid"`
	Fc  int    `json:"fc"`
	S   int64  `json:"s"`
	Pc  string `json:"pc"`
	Sha string `json:"sha"`
}

type Cloud115RspFiles struct {
	State bool              `json:"state"`
	Data  []Cloud115FileEntry `json:"data"`
	Count int               `json:"count"`
	Error string            `json:"error"`
}

type Cloud115RspDownURL struct {
	State   bool   `json:"state"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    map[string]struct {
		URL struct {
			URL    string `json:"url"`
			Client int    `json:"client"`
		} `json:"url"`
	} `json:"data"`
}

type Cloud115RspRename struct {
	State bool   `json:"state"`
	Error string `json:"error"`
}

type Cloud115RspShareFiles struct {
	State bool              `json:"state"`
	Error string            `json:"error"`
	Data  struct {
		List  []Cloud115FileEntry `json:"list"`
		Count int                `json:"count"`
	} `json:"data"`
}

type Cloud115RspVideoPreview struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		DriveID string `json:"drive_id"`
		FileID  string `json:"file_id"`
		VideoPreviewPlayInfo struct {
			Category string `json:"category"`
			LiveTranscodingSubtitleTaskList []struct {
				Language string `json:"language"`
				Status   string `json:"status"`
				URL      string `json:"url"`
			} `json:"live_transcoding_subtitle_task_list"`
			LiveTranscodingTaskList []struct {
				Stage          string `json:"stage"`
				Status         string `json:"status"`
				TemplateHeight int    `json:"template_height"`
				TemplateID     string `json:"template_id"`
				TemplateName   string `json:"template_name"`
				TemplateWidth  int    `json:"template_width"`
				URL            string `json:"url"`
			} `json:"live_transcoding_task_list"`
			Meta struct {
				Duration float64 `json:"duration"`
				Height   int     `json:"height"`
				Width    int     `json:"width"`
			} `json:"meta"`
		} `json:"video_preview_play_info"`
	} `json:"data"`
}

type Cloud115OpenForm struct {
	File       string `json:"file"`
	GalleryUid string `json:"gallery_uid"`
}

type Cloud115OpenVideo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		DriveID string `json:"drive_id"`
		FileID  string `json:"file_id"`
		VideoPreviewPlayInfo struct {
			Category string `json:"category"`
			LiveTranscodingSubtitleTaskList []struct {
				Language string `json:"language"`
				Status   string `json:"status"`
				URL      string `json:"url"`
			} `json:"live_transcoding_subtitle_task_list"`
			LiveTranscodingTaskList []struct {
				Stage          string `json:"stage"`
				Status         string `json:"status"`
				TemplateHeight int    `json:"template_height"`
				TemplateID     string `json:"template_id"`
				TemplateName   string `json:"template_name"`
				TemplateWidth  int    `json:"template_width"`
				URL            string `json:"url"`
			} `json:"live_transcoding_task_list"`
			Meta struct {
				Duration float64 `json:"duration"`
				Height   int     `json:"height"`
				Width    int     `json:"width"`
			} `json:"meta"`
		} `json:"video_preview_play_info"`
	} `json:"data"`
}

type Cloud115RspQRCode struct {
	State bool `json:"state"`
	Data  struct {
		UID     string `json:"uid"`
		Qrcode  string `json:"qrcode"`
		Time    int64  `json:"time"`
		Sign    string `json:"sign"`
		Expires int    `json:"expires"`
	} `json:"data"`
	Error string `json:"error"`
}

type Cloud115RspQRStatus struct {
	State bool `json:"state"`
	Data  struct {
		Status int `json:"status"`
	} `json:"data"`
}

type Cloud115RspQRLogin struct {
	State bool `json:"state"`
	Code  int  `json:"code"`
	Data  struct {
		Cookie map[string]string `json:"cookie"`
	} `json:"data"`
	Error string `json:"error"`
}
