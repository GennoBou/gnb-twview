package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/adrg/xdg"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// TwitchClientID is a default developer client ID for gnb-twview.
// This is not a secret, but is needed to interact with Twitch Helix API.
const TwitchClientID = "anttvy2w8s2xs7vzyfon07xvcm0xhn"

type TokenData struct {
	AccessToken string `json:"access_token"`
}

type TwitchUser struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
}

type TwitchUsersResponse struct {
	Data []TwitchUser `json:"data"`
}

type StreamInfo struct {
	UserName    string `json:"user_name"`
	UserLogin   string `json:"user_login"`
	GameName    string `json:"game_name"`
	ViewerCount int    `json:"viewer_count"`
	Title       string `json:"title"`
}

type TwitchStreamsResponse struct {
	Data []StreamInfo `json:"data"`
}

type DeviceAuthInfo struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
}

type TwitchDeviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type TwitchService struct {
	mu            sync.Mutex
	app           *application.App
	accessToken   string
	currentUser   *TwitchUser
	liveStreamers []StreamInfo

	// Queue management
	watchedTimes     map[string]int       // maps username to watched seconds
	lastWatched      map[string]time.Time // 各チャンネルの最終視聴日時
	initialWatchTime int                  // Target watch time per channel (e.g. 600s)
	rotationTime     int                  // Regular rotation interval (e.g. 300s)
	currentChannel   string
	autoMode         bool
	timeRemaining    int // Remaining seconds for current channel
	queue            []string

	// Device Auth control
	cancelAuth context.CancelFunc

	// Settings
	autoStartOnLogin bool
	ownerSet         bool
	language         string

	// Sub Windows
	mainWindow   *application.WebviewWindow
	twitchWindow *application.WebviewWindow
	authWindow   *application.WebviewWindow

	// Settings State
	settingsOpen   bool
	settingsWindow *application.WebviewWindow
	// Injection Texts
	injectCSS   string
	injectHTML  string
	injectJS    string
	injectTimer *time.Timer
}

func NewTwitchService() *TwitchService {
	s := &TwitchService{
		watchedTimes:     make(map[string]int),
		lastWatched:      make(map[string]time.Time),
		initialWatchTime: 360,  // 6分 (360秒)
		rotationTime:     180,  // 3分 (180秒)
		autoStartOnLogin: true, // デフォルトON
		ownerSet:         false,
		settingsOpen:     false,
		language:         "en", // デフォルト
	}
	s.loadInjectionFiles()
	s.loadSettings()
	s.loadWatchedTimes()
	return s
}

// SetApp saves the Wails application reference for events and window control.
func (s *TwitchService) SetApp(app *application.App) {
	s.app = app
}

func (s *TwitchService) SetWindows(main, twitch *application.WebviewWindow) {
	s.mu.Lock()
	s.mainWindow = main
	s.twitchWindow = twitch
	s.mu.Unlock()

	// 外部ドメイン（twitch.tv）をロードするため、Wails runtimeのreadyシグナルをGo側から手動で模倣する。
	// これにより w.runtimeLoaded が true となり、ExecJSが即座に動作するようになります。
	twitch.HandleMessage("wails:runtime:ready")

	// ロード完了時のイベント登録
	if runtime.GOOS == "windows" {
		log.Printf("[DEBUG] Registering WebViewNavigationCompleted for Windows\n")
		twitch.OnWindowEvent(events.Windows.WebViewNavigationCompleted, func(e *application.WindowEvent) {
			log.Printf("[DEBUG] WebViewNavigationCompleted event received\n")
			s.scheduleInjection()
		})
	} else if runtime.GOOS == "darwin" {
		log.Printf("[DEBUG] Registering WebViewDidFinishNavigation for macOS\n")
		twitch.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(e *application.WindowEvent) {
			log.Printf("[DEBUG] WebViewDidFinishNavigation event received\n")
			s.scheduleInjection()
		})
	}
}

func (s *TwitchService) scheduleInjection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	isLoggedIn := s.accessToken != "" && s.currentUser != nil
	currentChannel := s.currentChannel
	if !isLoggedIn || currentChannel == "" {
		return
	}

	if s.injectTimer != nil {
		s.injectTimer.Stop()
	}

	s.injectTimer = time.AfterFunc(300*time.Millisecond, func() {
		s.reapplyInjections()
	})
}

func (s *TwitchService) reapplyInjections() {
	s.mu.Lock()
	isLoggedIn := s.accessToken != "" && s.currentUser != nil
	currentChannel := s.currentChannel
	win := s.twitchWindow
	css := s.injectCSS
	html := s.injectHTML
	js := s.injectJS
	s.mu.Unlock()

	if win == nil {
		return
	}

	if !isLoggedIn || currentChannel == "" {
		return
	}

	log.Printf("[DEBUG] Reapplying injections (CSS len: %d, HTML len: %d, JS len: %d)\n", len(css), len(html), len(js))

	// CSS/HTMLの自己修復機能と、JSの実行を含むラッパーJSコード
	script := fmt.Sprintf(`
		(function() {
			// 1. CSSのインジェクション
			function applyStyle() {
				let style = document.getElementById('gnb-custom-style');
				const cssText = %q;
				if (!cssText) {
					if (style) style.remove();
					return;
				}
				if (!style) {
					style = document.createElement('style');
					style.id = 'gnb-custom-style';
					document.head.appendChild(style);
					console.log("[GNB] Created custom style element.");
				}
				if (style.textContent !== cssText) {
					style.textContent = cssText;
					console.log("[GNB] Applied custom CSS.");
				}
			}

			// 2. HTMLのインジェクション
			function applyHTML() {
				let container = document.getElementById('gnb-custom-html');
				const rawHTML = %q;
				if (!rawHTML) {
					if (container) container.remove();
					return;
				}
				if (!container) {
					container = document.createElement('div');
					container.id = 'gnb-custom-html';
					container.style.position = 'absolute';
					container.style.top = '0';
					container.style.left = '0';
					container.style.width = '100%%';
					container.style.height = '100%%';
					container.style.pointerEvents = 'none'; // 子要素（ボタンなど）のみクリックできるようにする
					container.style.zIndex = '99999';
					document.body.appendChild(container);
					console.log("[GNB] Created custom HTML container.");
				}
				if (container.innerHTML !== rawHTML) {
					container.innerHTML = rawHTML;
					console.log("[GNB] Applied custom HTML.");
				}
			}

			// 初回適用
			applyStyle();
			applyHTML();

			// 3. JSのインジェクション
			const jsCode = %q;
			if (jsCode) {
				try {
					const f = new Function(jsCode);
					f();
					console.log("[GNB] Executed custom JS.");
				} catch(err) {
					console.error("[GNB-ERROR] Custom JS execution failed:", err);
				}
			}

			// DOMの変更を監視し、消えたら再適用する自己修復機能
			if (!window._gnbObserver) {
				window._gnbObserver = new MutationObserver(function(mutations) {
					applyStyle();
					applyHTML();
				});
				window._gnbObserver.observe(document.documentElement, { childList: true, subtree: true });
				
				// 保険のポーリング
				setInterval(function() {
					applyStyle();
					applyHTML();
				}, 1500);
			}
		})();
		//# sourceURL=gnb-inject.js
	`, css, html, js)

	win.ExecJS(script)
}

func (s *TwitchService) syncSubWindows() {
	s.mu.Lock()
	twitchWin := s.twitchWindow
	channel := s.currentChannel
	s.mu.Unlock()

	if twitchWin == nil {
		return
	}
	if channel == "" {
		twitchWin.Hide()
		return
	}

	twitchURL := fmt.Sprintf("https://www.twitch.tv/%s", channel)

	log.Printf("[DEBUG] Syncing sub window for channel: %s\n", channel)

	twitchWin.SetURL(twitchURL)

	// Immediately synchronize the position and visibility of sub-windows
	s.SyncSubWindowPositions()
}

func (s *TwitchService) SyncSubWindowPositions() {
	s.mu.Lock()
	isLoggedIn := s.accessToken != "" && s.currentUser != nil
	currentChannel := s.currentChannel
	mainWin := s.mainWindow
	twitchWin := s.twitchWindow
	ownerSet := s.ownerSet
	s.mu.Unlock()

	if mainWin == nil || twitchWin == nil {
		return
	}

	if !isLoggedIn || currentChannel == "" {
		twitchWin.Hide()
		return
	}

	// Lazy initialize OS-level window ownership when HWNDs become ready
	if !ownerSet {
		mainHWND := uintptr(mainWin.NativeWindow())
		twitchHWND := uintptr(twitchWin.NativeWindow())

		if mainHWND != 0 && twitchHWND != 0 {
			setWindowOwner(twitchHWND, mainHWND)

			s.mu.Lock()
			s.ownerSet = true
			s.mu.Unlock()

			log.Printf("[DEBUG] Successfully initialized Z-order owner mapping on first syncSubWindowPositions\n")
		} else {
			log.Printf("[DEBUG] HWNDs not ready yet: main %x, twitch %x. Retrying next sync.\n", mainHWND, twitchHWND)
		}
	}

	x, y := mainWin.Position()
	w, h := mainWin.Size()

	// Windows 11 / 10 non-client border & shadow adjustments
	borderX := 8   // Horizontal shift (moves subwindows right to align with client area)
	borderY := 36  // Vertical shift (moves subwindows down)
	adjustW := -16 // Width adjustment (compensates for borderX on both sides)
	adjustH := -44 // Height adjustment (compensates for bottom border)

	// Apply adjustments
	adjX := x + borderX
	adjY := y + borderY
	adjW := w + adjustW
	adjH := h + adjustH

	if adjW < 0 {
		adjW = 0
	}
	if adjH < 0 {
		adjH = 0
	}

	// Titlebar / Navbar height
	navHeight := 50
	contentHeight := adjH - navHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	twitchWin.SetPosition(adjX, adjY+navHeight)
	twitchWin.SetSize(adjW, contentHeight)
	twitchWin.Show()
}

func (s *TwitchService) SetSettingsOpen(open bool) {
	s.mu.Lock()
	s.settingsOpen = open
	settingsWin := s.settingsWindow
	s.mu.Unlock()

	log.Printf("[DEBUG] SetSettingsOpen called: open=%t, settingsWinExists=%t\n", open, settingsWin != nil)

	if open {
		if settingsWin == nil {
			log.Printf("[DEBUG] settingsWindow is nil. Creating a new one.\n")
			if s.app != nil {
				title := "Settings"
				if s.language == "ja" {
					title = "設定"
				}
				settingsWin = s.app.Window.NewWithOptions(application.WebviewWindowOptions{
					Title:            title,
					URL:              "/?mode=settings",
					Width:            1080,
					Height:           648,
					MinWidth:         1080,
					MinHeight:        648,
					AlwaysOnTop:      true,
					BackgroundColour: application.NewRGB(10, 14, 23), // Dark Slate
				})

				settingsWin.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
					log.Printf("[DEBUG] settingsWindow WindowClosing event received in dynamic handler. Clearing reference.\n")
					s.mu.Lock()
					s.settingsWindow = nil
					s.settingsOpen = false
					s.mu.Unlock()
				})

				s.mu.Lock()
				s.settingsWindow = settingsWin
				s.mu.Unlock()
			}
		}

		if settingsWin != nil {
			log.Printf("[DEBUG] Showing settingsWindow\n")
			settingsWin.Show()
			settingsWin.Focus()
		}
	} else {
		if settingsWin != nil {
			log.Printf("[DEBUG] Hiding settingsWindow\n")
			settingsWin.Close()

			s.mu.Lock()
			s.settingsWindow = nil
			s.mu.Unlock()
		}
	}
}

func (s *TwitchService) SetCurrentChannel(channel string) {
	s.mu.Lock()
	if s.currentChannel == channel {
		s.mu.Unlock()
		return
	}
	s.currentChannel = channel
	s.mu.Unlock()

	s.syncSubWindows()
}

// getSystemLanguage は OS のデフォルトロケール名から言語コードを取得し、
// ja の場合は "ja"、それ以外は "en"（フォールバック）を返します。
func getSystemLanguage() string {
	if runtime.GOOS == "windows" {
		var buf [85]uint16
		mod := syscall.NewLazyDLL("kernel32.dll")
		proc := mod.NewProc("GetUserDefaultLocaleName")
		r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r != 0 {
			locale := syscall.UTF16ToString(buf[:])
			if len(locale) >= 2 {
				lang := locale[:2]
				if lang == "ja" {
					return "ja"
				}
			}
		}
	}
	return "en"
}

func (s *TwitchService) configDirPath() string {
	return filepath.Join(xdg.ConfigHome, "gnb-twview")
}

func (s *TwitchService) tokenFilePath() string {
	return filepath.Join(s.configDirPath(), "token.json")
}

func (s *TwitchService) settingsFilePath() string {
	return filepath.Join(s.configDirPath(), "settings.json")
}

func (s *TwitchService) watchedTimesFilePath() string {
	return filepath.Join(s.configDirPath(), "watched_times.json")
}

type WatchedTimeRecord struct {
	WatchedSeconds int       `json:"watched_seconds"`
	LastWatchedAt  time.Time `json:"last_watched_at"`
}

func (s *TwitchService) loadSettings() {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.settingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		// 設定ファイルがない場合はシステムの言語をデフォルト値として保存
		s.language = getSystemLanguage()
		s.saveSettingsLocked()
		return
	}

	var settings struct {
		RotationTime     int    `json:"rotation_time"`
		AutoStartOnLogin bool   `json:"auto_start_on_login"`
		Language         string `json:"language"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("[WARN] Failed to parse settings file: %v\n", err)
		s.language = getSystemLanguage()
		return
	}

	s.rotationTime = settings.RotationTime
	s.autoStartOnLogin = settings.AutoStartOnLogin

	// 言語設定がない、もしくは空の場合はシステムの言語を設定して保存
	if settings.Language == "" {
		s.language = getSystemLanguage()
		s.saveSettingsLocked()
	} else {
		s.language = settings.Language
	}
}

func (s *TwitchService) saveSettingsLocked() {
	path := s.settingsFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	settings := struct {
		RotationTime     int    `json:"rotation_time"`
		AutoStartOnLogin bool   `json:"auto_start_on_login"`
		Language         string `json:"language"`
	}{
		RotationTime:     s.rotationTime,
		AutoStartOnLogin: s.autoStartOnLogin,
		Language:         s.language,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func (s *TwitchService) loadWatchedTimes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.watchedTimesFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var records map[string]WatchedTimeRecord
	if err := json.Unmarshal(data, &records); err != nil {
		log.Printf("[WARN] Failed to parse watched times: %v\n", err)
		return
	}

	now := time.Now()
	for login, rec := range records {
		// 3時間以内のデータのみ復帰する
		if now.Sub(rec.LastWatchedAt) <= 3*time.Hour {
			s.watchedTimes[login] = rec.WatchedSeconds
			s.lastWatched[login] = rec.LastWatchedAt
		}
	}
}

func (s *TwitchService) saveWatchedTimesLocked() {
	path := s.watchedTimesFilePath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)

	records := make(map[string]WatchedTimeRecord)
	now := time.Now()

	// 保存時に3時間以上経過している古いデータをクレンジング（削除）する
	for login, sec := range s.watchedTimes {
		lastTime, exists := s.lastWatched[login]
		if !exists {
			lastTime = now
		}

		if now.Sub(lastTime) <= 3*time.Hour {
			records[login] = WatchedTimeRecord{
				WatchedSeconds: sec,
				LastWatchedAt:  lastTime,
			}
		} else {
			delete(s.watchedTimes, login)
			delete(s.lastWatched, login)
		}
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func (s *TwitchService) SaveWatchedTimes() {
	s.mu.Lock()
	s.saveWatchedTimesLocked()
	s.mu.Unlock()
}

func (s *TwitchService) loadInjectionFiles() {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.configDirPath()

	// CSS
	cssPath := filepath.Join(dir, "inject-css.txt")
	if data, err := os.ReadFile(cssPath); err == nil {
		s.injectCSS = string(data)
	}

	// HTML
	htmlPath := filepath.Join(dir, "inject-html.txt")
	if data, err := os.ReadFile(htmlPath); err == nil {
		s.injectHTML = string(data)
	}

	// JS
	jsPath := filepath.Join(dir, "inject-js.txt")
	if data, err := os.ReadFile(jsPath); err == nil {
		s.injectJS = string(data)
	}
}

func (s *TwitchService) GetInjectionTexts() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]string{
		"css":  s.injectCSS,
		"html": s.injectHTML,
		"js":   s.injectJS,
	}
}

func (s *TwitchService) SaveInjectionTexts(css, html, js string) {
	s.mu.Lock()
	s.injectCSS = css
	s.injectHTML = html
	s.injectJS = js
	s.mu.Unlock()

	dir := s.configDirPath()
	_ = os.MkdirAll(dir, 0755)

	_ = os.WriteFile(filepath.Join(dir, "inject-css.txt"), []byte(css), 0644)
	_ = os.WriteFile(filepath.Join(dir, "inject-html.txt"), []byte(html), 0644)
	_ = os.WriteFile(filepath.Join(dir, "inject-js.txt"), []byte(js), 0644)

	// 変更を現在開いているTwitchウィンドウへ再注入して即時反映させる
	s.reapplyInjections()
}

// LoadSavedToken loads the token from disk if it exists.
func (s *TwitchService) LoadSavedToken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.tokenFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[DEBUG] No saved token file found: %v\n", err)
		return false
	}

	var td TokenData
	if err := json.Unmarshal(data, &td); err != nil || td.AccessToken == "" {
		fmt.Printf("[DEBUG] Failed to unmarshal saved token: %v\n", err)
		return false
	}

	s.accessToken = td.AccessToken
	fmt.Printf("[DEBUG] Saved token found, validating...\n")

	// Try fetching user profile to validate token
	go func() {
		user, err := s.fetchUserProfile(td.AccessToken)
		if err == nil {
			s.mu.Lock()
			s.currentUser = user
			s.mu.Unlock()
			fmt.Printf("[DEBUG] Saved token valid, logged in as: %s\n", user.DisplayName)
			s.app.Event.Emit("login-status-changed", map[string]interface{}{
				"logged_in":    true,
				"display_name": user.DisplayName,
			})
			s.RefreshLiveStreamers()
			s.SyncSubWindowPositions()
		} else {
			fmt.Printf("[DEBUG] Saved token invalid, removing file: %v\n", err)
			// Token invalid, clear it
			s.mu.Lock()
			s.accessToken = ""
			s.mu.Unlock()
			_ = os.Remove(path)
			s.app.Event.Emit("login-status-changed", map[string]interface{}{
				"logged_in": false,
			})
			s.SyncSubWindowPositions()
		}
	}()

	return true
}

func (s *TwitchService) GetLoginStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken == "" || s.currentUser == nil {
		return map[string]interface{}{
			"logged_in": false,
		}
	}
	return map[string]interface{}{
		"logged_in":    true,
		"display_name": s.currentUser.DisplayName,
	}
}

// Login starts the Device Code Grant Flow and begins polling for token.
func (s *TwitchService) Login() (*DeviceAuthInfo, error) {
	s.mu.Lock()
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	// Close existing auth window if any
	if s.authWindow != nil {
		win := s.authWindow
		s.authWindow = nil
		go win.Close()
	}
	s.mu.Unlock()

	// Request device code
	data := url.Values{}
	data.Set("client_id", TwitchClientID)
	data.Set("scope", "user:read:follows")

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/device", data)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device auth request returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var dResp TwitchDeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&dResp); err != nil {
		return nil, fmt.Errorf("failed to decode device auth response: %w", err)
	}

	// Prepare polling context
	s.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelAuth = cancel

	// Create Auth Window inside the app
	authWin := s.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Twitch Login - gnb-twview",
		URL:    dResp.VerificationURI,
		Width:  500,
		Height: 650,
	})
	s.authWindow = authWin
	s.mu.Unlock()

	// Handle window close manually by user
	authWin.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		s.CancelLogin()
	})

	authWin.Center()
	authWin.Show()

	// Start polling in background
	go s.pollForToken(ctx, dResp)

	return &DeviceAuthInfo{
		UserCode:        dResp.UserCode,
		VerificationURI: dResp.VerificationURI,
	}, nil
}

func (s *TwitchService) pollForToken(ctx context.Context, dResp TwitchDeviceAuthResponse) {
	interval := dResp.Interval
	if interval <= 0 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	expiresIn := dResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1800
	}
	timeout := time.After(time.Duration(expiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			s.app.Event.Emit("device-auth-status", map[string]interface{}{
				"status": "expired",
				"error":  "認証の有効期限が切れました。もう一度やり直してください。",
			})
			s.closeAuthWindow()
			return
		case <-ticker.C:
			tokenData := url.Values{}
			tokenData.Set("client_id", TwitchClientID)
			tokenData.Set("device_code", dResp.DeviceCode)
			tokenData.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

			resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", tokenData)
			if err != nil {
				fmt.Printf("[DEBUG] Token request network error: %v\n", err)
				continue
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			fmt.Printf("[DEBUG] Token response status: %d, body: %s\n", resp.StatusCode, string(bodyBytes))

			if resp.StatusCode == http.StatusOK {
				var tr struct {
					AccessToken string `json:"access_token"`
				}
				if err := json.Unmarshal(bodyBytes, &tr); err == nil {
					// Wait 1 second for token propagation on Twitch servers
					time.Sleep(1 * time.Second)

					var user *TwitchUser
					var fetchErr error
					for attempt := 1; attempt <= 3; attempt++ {
						log.Printf("[DEBUG] Fetching user profile (attempt %d/3)...\n", attempt)
						user, fetchErr = s.fetchUserProfile(tr.AccessToken)
						if fetchErr == nil {
							break
						}
						log.Printf("[WARN] Fetch user profile attempt %d failed: %v\n", attempt, fetchErr)
						if attempt < 3 {
							time.Sleep(1 * time.Second)
						}
					}

					if fetchErr == nil && user != nil {
						s.mu.Lock()
						s.accessToken = tr.AccessToken
						s.currentUser = user
						s.mu.Unlock()

						path := s.tokenFilePath()
						_ = os.MkdirAll(filepath.Dir(path), 0755)
						tokenJson, _ := json.Marshal(TokenData{AccessToken: tr.AccessToken})
						_ = os.WriteFile(path, tokenJson, 0600)

						s.app.Event.Emit("login-status-changed", map[string]interface{}{
							"logged_in":    true,
							"display_name": user.DisplayName,
						})
						s.app.Event.Emit("device-auth-status", map[string]interface{}{
							"status": "success",
						})
						s.RefreshLiveStreamers()
						s.SyncSubWindowPositions()
						s.closeAuthWindow()
						return
					}

					// Exit polling and close window if profile cannot be fetched to prevent infinite polling loop
					log.Printf("[ERROR] Failed to fetch user profile after 3 attempts: %v. Aborting login.\n", fetchErr)
					s.app.Event.Emit("device-auth-status", map[string]interface{}{
						"status": "error",
						"error":  fmt.Sprintf("プロフィールの取得に失敗しました: %v", fetchErr),
					})
					s.closeAuthWindow()
					return
				} else {
					fmt.Printf("[DEBUG] JSON Decode error (success): %v\n", err)
					s.app.Event.Emit("device-auth-status", map[string]interface{}{
						"status": "error",
						"error":  fmt.Sprintf("トークン解析に失敗しました: %v", err),
					})
					s.closeAuthWindow()
					return
				}
			} else {
				var errResp struct {
					Error   string `json:"error"`
					Message string `json:"message"`
					Status  int    `json:"status"`
				}
				if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
					if errResp.Message == "authorization_pending" {
						continue
					} else if errResp.Message == "slow_down" {
						interval += 5
						ticker.Reset(time.Duration(interval) * time.Second)
					} else {
						s.app.Event.Emit("device-auth-status", map[string]interface{}{
							"status": "error",
							"error":  fmt.Sprintf("認証エラー: %s", errResp.Message),
						})
						s.closeAuthWindow()
						return
					}
				} else {
					fmt.Printf("[DEBUG] JSON Decode error (error): %v\n", err)
				}
			}
		}
	}
}

func (s *TwitchService) CancelLogin() {
	s.mu.Lock()
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	s.mu.Unlock()

	s.closeAuthWindow()
}

func (s *TwitchService) closeAuthWindow() {
	s.mu.Lock()
	win := s.authWindow
	s.authWindow = nil
	s.mu.Unlock()

	if win != nil {
		go win.Close()
	}
}

func (s *TwitchService) Logout() {
	s.mu.Lock()
	s.accessToken = ""
	s.currentUser = nil
	s.liveStreamers = nil
	s.autoMode = false
	s.currentChannel = ""
	twitchWin := s.twitchWindow
	s.mu.Unlock()

	_ = os.Remove(s.tokenFilePath())

	s.app.Event.Emit("login-status-changed", map[string]interface{}{
		"logged_in": false,
	})
	s.app.Event.Emit("streamers-updated", []StreamInfo{})
	s.SyncSubWindowPositions()

	// Clear session data and cookies on the WebView side
	if twitchWin != nil {
		clearJS := `
			(function() {
				const cookies = document.cookie.split(";");
				for (let i = 0; i < cookies.length; i++) {
					const cookie = cookies[i];
					const eqPos = cookie.indexOf("=");
					const name = eqPos > -1 ? cookie.substr(0, eqPos) : cookie;
					document.cookie = name + "=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/;domain=.twitch.tv";
					document.cookie = name + "=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/;domain=www.twitch.tv";
					document.cookie = name + "=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/";
				}
				localStorage.clear();
				sessionStorage.clear();
			})();
		`
		twitchWin.SetURL("https://www.twitch.tv/")
		twitchWin.ExecJS(clearJS)
	}
}

func (s *TwitchService) fetchUserProfile(token string) (*TwitchUser, error) {
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", TwitchClientID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("helix user profile returned HTTP status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var ur TwitchUsersResponse
	if err := json.Unmarshal(body, &ur); err != nil || len(ur.Data) == 0 {
		return nil, fmt.Errorf("failed to parse user profile: %w", err)
	}

	return &ur.Data[0], nil
}

// RefreshLiveStreamers fetches active live streams for followed channels.
func (s *TwitchService) RefreshLiveStreamers() {
	s.mu.Lock()
	token := s.accessToken
	user := s.currentUser
	s.mu.Unlock()

	if token == "" || user == nil {
		return
	}

	// Helix API to fetch followed streams
	apiURL := fmt.Sprintf("https://api.twitch.tv/helix/streams/followed?user_id=%s", user.ID)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", TwitchClientID)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching followed streams: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Helix followed streams API status %d\n", resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var sr TwitchStreamsResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		fmt.Printf("Error parsing streams json: %v\n", err)
		return
	}

	s.mu.Lock()
	s.liveStreamers = sr.Data
	s.mu.Unlock()

	s.app.Event.Emit("streamers-updated", sr.Data)

	// Rebuild and adjust active queue
	s.RebuildQueue()
}

// RebuildQueue constructs the active queue based on Smart Round-Robin rules.
func (s *TwitchService) RebuildQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.liveStreamers) == 0 {
		s.queue = []string{}
		return
	}

	type queueItem struct {
		login       string
		watchedTime int
		isGoalMet   bool
	}

	items := make([]queueItem, len(s.liveStreamers))
	for i, st := range s.liveStreamers {
		wt := s.watchedTimes[st.UserLogin]
		items[i] = queueItem{
			login:       st.UserLogin,
			watchedTime: wt,
			isGoalMet:   wt >= s.initialWatchTime,
		}
	}

	// Smart Sort:
	// 1. Goal Unmet comes first.
	// 2. Sort by watchedTime ascending (least watched first).
	sort.Slice(items, func(i, j int) bool {
		if items[i].isGoalMet != items[j].isGoalMet {
			return !items[i].isGoalMet // Unmet comes first
		}
		return items[i].watchedTime < items[j].watchedTime // Least watched first
	})

	// Rebuild queue
	newQueue := make([]string, len(items))
	for i, item := range items {
		newQueue[i] = item.login
	}
	s.queue = newQueue
}

// Service bindings for Frontend

func (s *TwitchService) GetSettings() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	autoStartVal := 0
	if s.autoStartOnLogin {
		autoStartVal = 1
	}
	return map[string]any{
		"initial_watch_time":  s.initialWatchTime,
		"rotation_time":       s.rotationTime,
		"auto_start_on_login": autoStartVal,
		"language":            s.language,
	}
}

func (s *TwitchService) SaveSettings(initial, rotation, autoStart int, language string) {
	s.mu.Lock()
	s.initialWatchTime = 360 // 6分(360秒)に固定
	s.rotationTime = rotation
	s.autoStartOnLogin = autoStart == 1
	s.language = language
	s.saveSettingsLocked()
	s.mu.Unlock()

	s.RebuildQueue()

	if s.app != nil {
		s.app.Event.Emit("settings-saved", map[string]any{
			"initial_watch_time":  s.initialWatchTime,
			"rotation_time":       rotation,
			"auto_start_on_login": autoStart,
			"language":            language,
		})
	}
}

func (s *TwitchService) StartAutoMode(currentChannel string) map[string]interface{} {
	s.mu.Lock()
	s.autoMode = true
	s.currentChannel = currentChannel

	if currentChannel != "" {
		s.lastWatched[currentChannel] = time.Now()
	}

	// If current channel is in live lists, initialize time remaining based on its watched status
	wt := s.watchedTimes[currentChannel]
	target := s.initialWatchTime
	if wt >= s.initialWatchTime {
		target = s.rotationTime
	} else {
		// 目標未達の場合は、目標時間からすでに視聴した時間を引いた残りの時間を設定する
		target = s.initialWatchTime - wt
	}

	s.timeRemaining = target
	s.mu.Unlock()

	s.RebuildQueue()
	s.syncSubWindows()

	return s.getAutoModeState()
}

func (s *TwitchService) StopAutoMode() {
	s.mu.Lock()
	s.autoMode = false
	s.mu.Unlock()
}

func (s *TwitchService) SkipStreamer() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return map[string]interface{}{"status": "queue empty"}
	}

	nextIdx := 0
	// If we are currently watching a channel, find next one in the queue
	if s.currentChannel != "" {
		for i, login := range s.queue {
			if login == s.currentChannel {
				nextIdx = (i + 1) % len(s.queue)
				break
			}
		}
	}

	s.currentChannel = s.queue[nextIdx]

	if s.currentChannel != "" {
		s.lastWatched[s.currentChannel] = time.Now()
	}

	wt := s.watchedTimes[s.currentChannel]
	target := s.initialWatchTime
	if wt >= s.initialWatchTime {
		target = s.rotationTime
	} else {
		// 目標未達の場合は、目標時間からすでに視聴した時間を引いた残りの時間を設定する
		target = s.initialWatchTime - wt
	}
	s.timeRemaining = target

	s.app.Event.Emit("streamer-switched", map[string]interface{}{
		"channel":        s.currentChannel,
		"time_remaining": s.timeRemaining,
	})

	s.saveWatchedTimesLocked()

	s.mu.Unlock()
	s.syncSubWindows()
	s.mu.Lock()

	return s.getAutoModeState()
}

func (s *TwitchService) TickSecond() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.autoMode || s.currentChannel == "" {
		return s.getAutoModeState()
	}

	// Increment watched seconds
	s.watchedTimes[s.currentChannel]++
	s.lastWatched[s.currentChannel] = time.Now()

	s.timeRemaining--
	if s.timeRemaining <= 0 {
		s.mu.Unlock()
		s.SkipStreamer()
		s.mu.Lock()
	}

	return s.getAutoModeState()
}

func (s *TwitchService) getAutoModeState() map[string]interface{} {
	return map[string]interface{}{
		"auto_mode":       s.autoMode,
		"current_channel": s.currentChannel,
		"time_remaining":  s.timeRemaining,
		"queue":           s.queue,
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", etc.
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procSetWindowLong    = user32.NewProc("SetWindowLongW")
	procGetAncestor      = user32.NewProc("GetAncestor")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
)

// getRootHWND gets the top-level parent window HWND for Wails window handles.
func getRootHWND(hwnd uintptr) uintptr {
	// GA_ROOT = 2 (retrieves the root window)
	r, _, _ := procGetAncestor.Call(hwnd, uintptr(2))
	if r != 0 {
		return r
	}
	return hwnd
}

// setWindowOwner sets the Win32 HWND owner relationship.
// This locks the child window's Z-order directly above the parent window.
func setWindowOwner(childHWND, parentHWND uintptr) {
	childRoot := getRootHWND(childHWND)
	parentRoot := getRootHWND(parentHWND)

	log.Printf("[DEBUG] setWindowOwner mapping: childHWND %x (root %x) -> parentHWND %x (root %x)\n", childHWND, childRoot, parentHWND, parentRoot)

	gwlHwndParent := ^uintptr(7) // GWL_HWNDPARENT = -8 (expressed via bitwise negation ^7)

	if err := procSetWindowLongPtr.Find(); err == nil {
		procSetWindowLongPtr.Call(childRoot, gwlHwndParent, parentRoot)
	} else {
		procSetWindowLong.Call(childRoot, gwlHwndParent, parentRoot)
	}

	// Update window frame and Z-order style cache.
	// SWP_NOSIZE(0x0001) | SWP_NOMOVE(0x0002) | SWP_NOZORDER(0x0004) | SWP_NOACTIVATE(0x0010) | SWP_FRAMECHANGED(0x0020) = 0x0037
	procSetWindowPos.Call(childRoot, 0, 0, 0, 0, 0, uintptr(0x0037))
}
