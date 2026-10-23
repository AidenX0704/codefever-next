package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"image/color"
	"math/big"
	mrand "math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"codefever-next/backend/internal/httpapi"
)

// CaptchaHandler 实现 captcha 控制器。
//
// 逐字对应 application/controllers/captcha.php:13-24：
// 生成 6 位随机文本 → 存进会话 → 输出 150x50 的 PNG 图片。
type CaptchaHandler struct{}

// NewCaptchaHandler 创建 captcha 控制器。
func NewCaptchaHandler() *CaptchaHandler { return &CaptchaHandler{} }

// RegisterWeb 把 captcha 控制器注册到服务器。
func (h *CaptchaHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "captcha",
		Methods: map[string]http.HandlerFunc{
			"view": h.view,
		},
	})
}

// captchaChars 是验证码字符表。
//
// 逐字对应 application/controllers/captcha.php:28：
// 'abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNOPQRSTUVWXYZ'（去掉了易混淆的 i、l、I、L）。
const captchaChars = "abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNOPQRSTUVWXYZ"

// captchaWidth 是验证码图片宽度。
//
// 逐字对应 application/controllers/captcha.php:21。
const captchaWidth = 150

// captchaHeight 是验证码图片高度。
//
// 逐字对应 application/controllers/captcha.php:22。
const captchaHeight = 50

// captchaTTL 是验证码文本的保存时长。
//
// PHP 侧存在 $_SESSION 里，生命周期与会话相同（10 天）。
const captchaTTL = 10 * time.Minute

// captchaCookieName 是携带验证码 id 的 Cookie 名字。
const captchaCookieName = "codefever_captcha"

// view 实现 captcha/view。
//
// 逐字对应 application/controllers/captcha.php:13-24。
func (h *CaptchaHandler) view(w http.ResponseWriter, r *http.Request) {
	text := generateCaptchaText(6)
	captchaID := h.remember(text)
	// PHP 侧验证码存在 $_SESSION 里，会话 id 由 PHPSESSID Cookie 携带；
	// 新后端用独立 Cookie 携带验证码 id，作用相同。
	if captchaID != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     captchaCookieName,
			Value:    captchaID,
			Path:     "/",
			MaxAge:   int(captchaTTL.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// PHP 侧用 Duality.ttf 渲染并给每个字符加随机旋转；
	// Go 侧不引入字体渲染依赖，改用内置 5x7 点阵字体 + 随机竖直抖动。
	img := newCanvas(captchaWidth, captchaHeight, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	rnd := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	const scale = 4
	x := 3
	for _, ch := range text {
		y := 11 + rnd.Intn(7) - 3
		drawText(img, string(ch), x, y, scale, randColor(rnd))
		x += (glyphWidth + 1) * scale
	}

	content, err := encodePNG(img)
	if err != nil {
		http.Error(w, "captcha error", http.StatusInternalServerError)
		return
	}

	// PHP 侧 Captcha.php:260-264 设置的响应头，逐字复刻。
	w.Header().Set("Pragma", "public")
	w.Header().Set("Expires", "0")
	w.Header().Set("Cache-Control", "must-revalidate, post-check=0, pre-check=0")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// generateCaptchaText 生成随机验证码文本。
//
// 逐字对应 application/controllers/captcha.php:26-36（_generateText）。
func generateCaptchaText(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(captchaChars))))
		if err != nil {
			// 随机源异常时退回到时间种子，保证验证码仍然可用。
			b[i] = captchaChars[(time.Now().UnixNano()+int64(i))%int64(len(captchaChars))]
			continue
		}
		b[i] = captchaChars[n.Int64()]
	}
	return string(b)
}

// captchaStore 保存最近生成的验证码文本。
//
// PHP 侧把验证码写进 $_SESSION['captcha']（application/controllers/captcha.php:18），
// 但**整个应用没有任何地方读取它** —— 注册与登录表单都不校验验证码。
// 因此新后端用进程内缓存保存即可：它只为将来可能的校验留个口子，
// 不影响任何可观察的响应。
var captchaStore = struct {
	sync.Mutex
	items map[string]captchaEntry
}{items: map[string]captchaEntry{}}

// captchaEntry 是一条验证码记录。
type captchaEntry struct {
	text    string
	expires time.Time
}

// remember 保存验证码文本，返回它的 id。
func (h *CaptchaHandler) remember(text string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	id := hex.EncodeToString(buf)

	captchaStore.Lock()
	defer captchaStore.Unlock()
	now := time.Now()
	for k, v := range captchaStore.items {
		if now.After(v.expires) {
			delete(captchaStore.items, k)
		}
	}
	captchaStore.items[id] = captchaEntry{text: text, expires: now.Add(captchaTTL)}
	return id
}
