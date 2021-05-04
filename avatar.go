package hostlocavatar

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hack-fan/x/xerr"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	_ "golang.org/x/image/webp"
)

var rest = resty.New().SetRetryCount(3).
	SetRetryWaitTime(5 * time.Second).
	SetRetryMaxWaitTime(60 * time.Second)

// Upload avatar to hostloc.com
// input and agent can getting from https://hostloc.com/home.php?mod=spacecp&ac=avatar
// search input= and agent=
// avatarURL is a online image url
func Upload(input, agent, avatarURL string) error {
	// 下载
	resp, err := rest.R().SetDoNotParseResponse(true).Get(avatarURL)
	if err != nil {
		return xerr.Newf(400, "InvalidAvatarURL", "请检查传入的头像链接是否正确, %s", err)
	}
	defer resp.RawBody().Close() // nolint
	// 解析图像
	avatar, _, err := image.Decode(resp.RawBody())
	if err != nil {
		return xerr.Newf(400, "InvalidAvatar", "不支持你传入的头像格式, %s", err)
	}
	// 切割正方形
	tmp, err := cutter.Crop(avatar, cutter.Config{
		Width:   1,
		Height:  1,
		Mode:    cutter.Centered,
		Options: cutter.Ratio & cutter.Copy,
	})
	if err != nil {
		return fmt.Errorf("切割正方形失败：%w", err)
	}
	// 从大到小缩小头像，tmp是个指针缩小后会被改变
	a200 := hexImage(200, tmp)
	a120 := hexImage(120, tmp)
	a48 := hexImage(48, tmp)
	// 上传
	return uploadAvatar(input, agent, a48, a120, a200)
}

// resize img to size and output it's hax string
// warning: img will be changed without a copy
func hexImage(size uint, img image.Image) string {
	thumbnail := resize.Thumbnail(size, size, img, resize.Lanczos2)
	buff := new(bytes.Buffer)
	// 如果不是 Image 前边早都报错了，这里是不会报错的
	_ = png.Encode(buff, thumbnail)
	return fmt.Sprintf("%X", buff.Bytes())
}

func uploadAvatar(input, agent, s, m, l string) error {
	url := "https://hostloc.com/uc_server/index.php?m=user&inajax=1&a=rectavatar&appid=1&input=" + input + "&agent=" + agent + "&avatartype=virtual"
	resp, err := rest.R().SetFormData(map[string]string{
		"avatar1":     l,
		"avatar2":     m,
		"avatar3":     s,
		"urlReaderTS": strconv.FormatInt(time.Now().Unix(), 10),
	}).Post(url)
	if err != nil {
		return err
	}
	switch resp.String() {
	case `<?xml version="1.0" ?><root><face success="1"/></root>`:
		// 成功
		return nil
	case `<?xml version="1.0" ?><root><face success="0"/></root>`:
		return xerr.Newf(500, "InvalidImage", "图片问题")
	case `<root><message type="error" value="-2" /></root>`:
		return xerr.Newf(500, "UploadFailed", "文件上传问题")
	case `Access denied for agent changed`:
		return xerr.Newf(400, "InvalidInput", "input 值过期了，请重新去刷新后打开页面源代码复制")
	case `Authorization has expired`:
		return xerr.Newf(400, "AuthFailed", "你在hostloc登录过期了，请重新登录后再获取input值")
	default:
		return fmt.Errorf("unknown error: %s", resp.String())
	}
}
