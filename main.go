package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	baseURL             = "https://www.kanqiulei.cc" // 更新了基础域名
	playlistFile        = "playlist.m3u"
	liveLinksFile       = "live_links.txt"
	logFile             = "scraper_log.txt"
	lockFile            = "task.lock"
	retentionHours      = 5  // 保留距现在前 5 小时
	timeWindowMinutes   = 60 // 抓取过去 60 分钟的比赛
	listFetchTimeout    = 10 * time.Second
	detailFetchTimeout  = 10 * time.Second
	listFetchRetries    = 3
	listRetryWait       = 2 * time.Second
	detailFetchRetries  = 2
	detailRetryWait     = 1 * time.Second
	maxDetailConcurrent = 6
)

var leagueShortNames = map[string]string{
	// ================= [足球 - 欧洲五大联赛及杯赛] =================
	"英格兰超级联赛":     "英超",
	"英格兰冠军联赛":     "英冠",
	"英格兰甲组联赛":     "英甲",
	"英格兰乙组联赛":     "英乙",
	"英格兰足球协会全国联赛": "英议联",
	"英格兰足总杯":      "足总杯",
	"英格兰联赛杯":      "英联杯",

	"西班牙甲组联赛": "西甲",
	"西班牙乙组联赛": "西乙",
	"西班牙国王杯":  "国王杯",
	"西班牙超级杯":  "西超杯",

	"意大利甲组联赛": "意甲",
	"意大利乙组联赛": "意乙",
	"意大利丙组联赛": "意丙",
	"意大利杯":    "意杯",
	"意大利超级杯":  "意超杯",

	"德国甲组联赛": "德甲",
	"德国乙组联赛": "德乙",
	"德国丙组联赛": "德丙",
	"德国杯":    "德国杯",
	"德国超级杯":  "德超杯",

	"法国甲组联赛":   "法甲",
	"法国乙组联赛":   "法乙",
	"法国丙组全国联赛": "法丙",
	"法国杯":      "法国杯",
	"法国超级杯":    "法超杯",

	// ================= [足球 - 欧洲洲际赛 & 欧洲其他主流] =================
	"欧洲冠军联赛": "欧冠",
	"欧足联欧洲联赛": "欧联",
	"欧洲联赛":   "欧联",
	"欧洲协会联赛": "欧协联",
	"欧洲超级杯":  "欧洲超级杯",
	"欧洲国家联赛": "欧国联",
	"欧洲青年联赛": "欧青联",

	"葡萄牙超级联赛": "葡超",
	"葡萄牙甲组联赛": "葡甲",
	"葡萄牙杯":    "葡萄牙杯",
	"荷兰甲组联赛":  "荷甲",
	"荷兰乙组联赛":  "荷乙",
	"苏格兰超级联赛": "苏超",
	"俄罗斯超级联赛": "俄超",
	"土耳其超级联赛": "土超",
	"比利时甲组联赛A": "比甲",
	"瑞士超级联赛":  "瑞士超",
	"瑞典超级联赛":  "瑞典超",
	"挪威超级联赛":  "挪超",
	"丹麦超级联赛":  "丹超",
	"奥地利甲组联赛": "奥甲",
	"希腊超级联赛甲组": "希超",
	"乌克兰超级联赛": "乌超",

	// ================= [足球 - 亚洲 & 大洋洲] =================
	"中国超级联赛": "中超",
	"中国甲组联赛": "中甲",
	"中国乙组联赛": "中乙",
	"中国足协杯":  "足协杯",
	"中国全运会":  "全运会",
	"香港超级联赛": "港超",

	"日本J1联赛": "日职联",
	"日本J2联赛": "日职乙",
	"日本J3联赛": "日丙",
	"日本天皇杯":  "天皇杯",
	"日本联赛杯":  "日联杯",

	"韩国K甲组联赛": "韩K联",
	"韩国K乙组联赛": "韩K2",
	"韩国足总杯":   "韩足总杯",

	"澳大利亚甲组联赛": "澳超",
	"澳大利亚足总杯":  "澳足总杯",

	"沙特超级联赛":  "沙特超",
	"伊朗超级联赛":  "伊朗超",
	"阿联酋超级联赛": "阿联酋超",
	"卡塔尔甲组联赛": "卡塔尔联",

	"亚足联冠军精英联赛": "亚冠精英",
	"亚足联冠军联赛二":  "亚冠2",
	"亚足联挑战联赛":   "亚挑联",
	"大洋洲足联职业联赛": "大洋联",

	// ================= [足球 - 美洲] =================
	"美国职业大联盟":  "美职联",
	"美国公开赛冠军杯": "美公开杯",

	"巴西甲组联赛": "巴甲",
	"巴西乙组联赛": "巴乙",
	"巴西丙组联赛": "巴丙",
	"巴西杯":    "巴西杯",

	"阿根廷职业联赛":     "阿甲",
	"阿根廷乙组曼特波里頓联赛": "阿乙",
	"阿根廷杯":        "阿根廷杯",
	"阿根廷职业联赛杯":    "阿联杯",

	"墨西哥超级联赛":  "墨超",
	"哥伦比亚甲组联赛": "哥甲",
	"智利甲组联赛":   "智甲",
	"秘鲁甲组联赛":   "秘鲁甲",

	"南美自由杯":       "解放者杯",
	"南美洲球会杯":      "南美杯",
	"南美超级杯":       "南美优胜者杯",
	"中北美洲及加勒比海冠军杯": "美冠杯",

	// ================= [足球 - 女足 & 国家队 & 友谊赛] =================
	"英格兰女子超级联赛": "英女超",
	"西班牙女子甲组联赛": "西女甲",
	"意大利女子甲组联赛": "意女甲",
	"德国女子甲组联赛":  "德女甲",
	"法国女子超级联赛":  "法女超",
	"欧洲女子冠军联赛":  "女足欧冠",

	"国际友谊赛":             "友谊赛",
	"俱乐部友谊赛":            "球会友谊",
	"女子国际友谊赛":           "女足友谊",
	"2026世界杯亚洲外围赛":      "世亚预",
	"2026世界杯欧洲外围赛":      "世欧预",
	"2026世界杯南美洲外围赛":     "世南美预",
	"2026世界杯中北美洲及加勒比海外围赛": "世北美预",
	"2026世界杯非洲外围赛":      "世非预",

	// ================= [篮球 - 豪华大满贯] =================
	"NBA美国篮球职业联赛":   "NBA",
	"NBA美国篮球职业联赛杯":  "NBA季中赛",
	"NBA美国篮球职业全明星赛": "NBA全明星",
	"女子美国职业篮球联赛":   "WNBA",
	"美国大学篮球联赛":     "NCAA",

	"CBA中国篮球职业联赛":   "CBA",
	"CBA中国篮球俱乐部杯":   "CBA俱乐部杯",
	"WCBA中国女子篮球职业联赛": "WCBA",
	"中国全国篮球联赛":     "中国NBL",
	
	"中华台北篮球P联盟职业联赛": "PLG",
	"中华台北篮球职业大联盟":  "T1联赛",
	"中华台北篮球超级联赛":   "SBL",

	"欧洲篮球冠军联赛":     "欧篮联",
	"FIBA欧洲篮球杯":    "欧篮杯",
	"欧洲亚得里亚海篮球甲组联赛": "亚海联",
	"VTB篮球联合赛":     "VTB杯",

	"西班牙篮球甲组联赛": "西篮甲",
	"西班牙篮球国王杯":  "西国王杯",
	"意大利篮球甲组联赛": "意篮甲",
	"法国篮球甲组联赛":  "法篮甲",
	"德国篮球甲组联赛":  "德篮甲",
	"土耳其篮球超级联赛": "土篮超",
	"希腊篮球甲组联赛":  "希篮甲",
	"俄罗斯篮球超级联赛": "俄篮超",

	"日本篮球B1联赛":   "日篮B1",
	"日本篮球B2联赛":   "日篮B2",
	"韩国篮球联赛":     "KBL",
	"韩国女子篮球联赛":   "WKBL",
	"菲律宾篮球PBA委员杯": "PBA",
	"菲律宾篮球马哈里卡联赛": "MPBL",
	"澳大利亚国家篮球联赛": "澳洲NBL",
	"东亚篮球超级联赛":   "东亚超",

	"阿根廷全国篮球联赛":     "阿篮甲",
	"巴西篮球联赛":       "巴篮甲",
	"FIBA篮球世界杯欧洲资格赛": "世预赛欧洲区",
	"FIBA篮球世界杯亚洲资格赛": "世预赛亚洲区",
	"FIBA篮球世界杯美洲预选赛": "世预赛美洲区",
	"篮球国际友谊赛":      "篮球友谊",
}

var (
	taskMu      sync.Mutex
	taskRunning bool
)

type matchCandidate struct {
	Title     string
	URL       string
	Time      string
	Timestamp time.Time
}

type item struct {
	Time      string
	Title     string
	URL       string
	Timestamp time.Time
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	go func() {
		if err := runScrape(context.Background()); err != nil {
			appendLog("startup task failed: " + err.Error())
		}
	}()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/trigger", triggerHandler)
	http.HandleFunc("/task", taskHandler)
	http.HandleFunc("/playlist.m3u", staticFileHandler(playlistFile, "audio/x-mpegurl"))
	http.HandleFunc("/live_links.txt", staticFileHandler(liveLinksFile, "text/plain; charset=utf-8"))
	http.HandleFunc("/scraper_log.txt", staticFileHandler(logFile, "text/plain; charset=utf-8"))

	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<h2>✅ 抓取服务运行中 (Go)</h2>
<ul>
<li><a href='/playlist.m3u' target='_blank'>📥 播放列表 (playlist.m3u)</a></li>
<li><a href='/live_links.txt' target='_blank'>📄 文本源 (live_links.txt)</a></li>
<li><a href='/scraper_log.txt' target='_blank'>📝 运行日志 (scraper_log.txt)</a></li>
<li><a href='/trigger' target='_blank'>🚀 手动触发抓取 (/trigger)</a></li>
</ul>
<p style='color:gray; font-size:12px;'>抓取任务在后台运行，不阻塞页面访问。</p>`)
}

func triggerHandler(w http.ResponseWriter, _ *http.Request) {
	if !startTaskIfIdle() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<h2>⏳ 已有任务在运行，请稍后再试。</h2>")
		return
	}
	go func() {
		defer finishTask()
		if err := runScrape(context.Background()); err != nil {
			appendLog("manual task failed: " + err.Error())
		}
	}()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<h2>🚀 抓取任务已后台启动</h2><p>1~2 分钟后查看 playlist.m3u。</p>")
}

func taskHandler(w http.ResponseWriter, _ *http.Request) {
	if !startTaskIfIdle() {
		http.Error(w, "task already running", http.StatusTooManyRequests)
		return
	}
	defer finishTask()
	if err := runScrape(context.Background()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = io.WriteString(w, "ok")
}

func staticFileHandler(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		b, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(b)
	}
}

func startTaskIfIdle() bool {
	taskMu.Lock()
	defer taskMu.Unlock()
	if taskRunning {
		return false
	}
	taskRunning = true
	return true
}

func finishTask() {
	taskMu.Lock()
	taskRunning = false
	taskMu.Unlock()
}

func runScrape(ctx context.Context) error {
	unlock, err := acquireFileLock(lockFile)
	if err != nil {
		appendLog("检测到已有抓取任务在运行，本次跳过")
		return nil
	}
	defer unlock()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	retentionCutoff := now.Add(-time.Duration(retentionHours) * time.Hour)
	timeWindowStart := now.Add(-timeWindowMinutes * time.Minute)

	appendLog("--- Go 抓取任务启动 ---")

	allItems, existingURLs, existingTitles := loadExistingItems(loc, now, retentionCutoff)

	// 👉 首页即默认的足球列表，篮球为 /lanqiu.html
	listURLs := []string{baseURL + "/", baseURL + "/lanqiu.html"}
	listBodies := fetchURLs(ctx, listURLs, listFetchTimeout, 2, listFetchRetries, listRetryWait, true)

	candidates := make([]matchCandidate, 0, 64)
	skipCount := 0
	for _, u := range listURLs {
		body := listBodies[u]
		if body == "" {
			appendLog("分类页抓取失败: " + u)
			continue
		}
		for _, c := range parseListCandidates(body, timeWindowStart, now, loc) {
			if existingTitles[c.Title] {
				skipCount++
				continue
			}
			candidates = append(candidates, c)
		}
	}
	appendLog(fmt.Sprintf("发现 %d 场候选，前置标题跳过 %d", len(candidates), skipCount))

	detailURLs := make([]string, 0, len(candidates))
	candByURL := make(map[string]matchCandidate, len(candidates))
	for _, c := range candidates {
		full := c.URL
		if !strings.HasPrefix(full, "http") {
			full = baseURL + full
		}
		detailURLs = append(detailURLs, full)
		candByURL[full] = c
	}
	detailBodies := fetchURLs(ctx, detailURLs, detailFetchTimeout, maxDetailConcurrent, detailFetchRetries, detailRetryWait, true)

	successCount := 0
	urlSkipCount := 0
	for _, du := range detailURLs {
		body := detailBodies[du]
		streamURL := extractM3U8(body)
		if streamURL == "" {
			continue
		}
		cleanURL := normalizeStreamURL(streamURL)
		if cleanURL == "" {
			continue
		}
		if existingURLs[cleanURL] {
			urlSkipCount++
			continue
		}
		c := candByURL[du]

		allItems = append(allItems, item{
			Time:      c.Time,
			Title:     c.Title,
			URL:       cleanURL,
			Timestamp: c.Timestamp,
		})
		existingURLs[cleanURL] = true
		successCount++
	}

	sort.Slice(allItems, func(i, j int) bool {
		diffI := now.Sub(allItems[i].Timestamp).Abs()
		diffJ := now.Sub(allItems[j].Timestamp).Abs()
		return diffI < diffJ
	})

	if err := writeOutputs(allItems, now.Format("2006-01-02")); err != nil {
		return err
	}
	appendLog(fmt.Sprintf("任务完成：新增 %d，标题跳过 %d，URL 跳过 %d", successCount, skipCount, urlSkipCount))
	return nil
}

func acquireFileLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, err
		}
		return nil, err
	}
	_, _ = f.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	unlock := func() {
		_ = f.Close()
		_ = os.Remove(path)
	}
	return unlock, nil
}

func appendLog(msg string) {
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	fmt.Print(line)
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func loadExistingItems(loc *time.Location, now, retentionCutoff time.Time) ([]item, map[string]bool, map[string]bool) {
	items := make([]item, 0, 256)
	existingURLs := map[string]bool{}
	existingTitles := map[string]bool{}

	b, err := os.ReadFile(playlistFile)
	if err != nil {
		return items, existingURLs, existingTitles
	}
	s := bufio.NewScanner(bytes.NewReader(b))
	lines := []string{}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	infRe := regexp.MustCompile(`group-title="([^"]+)", \[(\d{2}):(\d{2})\] (.+)$`)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "#EXTINF") {
			continue
		}
		if i+1 >= len(lines) {
			break
		}
		url := lines[i+1]
		i++
		m := infRe.FindStringSubmatch(line)
		if len(m) != 5 {
			continue
		}

		timeStr := m[2] + ":" + m[3]
		title := m[4]

		dateStr := now.Format("2006-01-02")
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr+" "+timeStr+":00", loc)
		if err == nil {
			if ts.Sub(now) > 12*time.Hour {
				ts = ts.AddDate(0, 0, -1)
			} else if now.Sub(ts) > 12*time.Hour {
				ts = ts.AddDate(0, 0, 1)
			}
		}

		if err != nil || ts.Before(retentionCutoff) {
			continue
		}
		items = append(items, item{
			Time:      timeStr,
			Title:     title,
			URL:       url,
			Timestamp: ts,
		})
		existingURLs[url] = true
		existingTitles[title] = true
	}
	return items, existingURLs, existingTitles
}

func fetchURLs(
	ctx context.Context,
	urls []string,
	timeout time.Duration,
	maxConcurrent int,
	retries int,
	wait time.Duration,
	logRetry bool,
) map[string]string {
	if retries < 1 {
		retries = 1
	}
	if wait < 0 {
		wait = 0
	}

	results := make(map[string]string, len(urls))
	if len(urls) == 0 {
		return results
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	client := &http.Client{Timeout: timeout}
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, rawURL := range urls {
		u := rawURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			for attempt := 1; attempt <= retries; attempt++ {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
				if err != nil {
					return
				}
				req.Header.Set("User-Agent", "Mozilla/5.0")
				req.Header.Set("Accept-Encoding", "gzip")
				req.Header.Set("Referer", baseURL+"/") // 👉 增加防盗链规避

				resp, err := client.Do(req)
				if err == nil {
					body := ""
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						body, err = readMaybeGzip(resp.Body, resp.Header.Get("Content-Encoding"))
					}
					resp.Body.Close()
					if err == nil && body != "" {
						mu.Lock()
						results[u] = body
						mu.Unlock()
						return
					}
				}

				if attempt < retries && wait > 0 {
					if logRetry {
						appendLog(fmt.Sprintf("请求失败，等待后重试 (%d/%d)", attempt, retries))
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(wait):
					}
				}
			}
		}()
	}

	wg.Wait()
	return results
}

func readMaybeGzip(r io.Reader, encoding string) (string, error) {
	if strings.Contains(strings.ToLower(encoding), "gzip") {
		gr, err := gzip.NewReader(r)
		if err != nil {
			return "", err
		}
		defer gr.Close()
		b, err := io.ReadAll(gr)
		return string(b), err
	}
	b, err := io.ReadAll(r)
	return string(b), err
}

func shortenLeagueName(name string) (string, bool) {
	for full, short := range leagueShortNames {
		if strings.Contains(name, full) {
			return strings.ReplaceAll(name, full, short), true
		}
	}
	return name, false
}

func parseListCandidates(html string, winStart, now time.Time, loc *time.Location) []matchCandidate {
	tagRe := regexp.MustCompile(`(?is)<div class="panel">.*?<span class="title">[^<]*</span>`)
	timeRe := regexp.MustCompile(`(?is)<span class="time">\s*(\d{2}-\d{2})\s+(\d{2}:\d{2})\s*</span>`)
	hrefRe := regexp.MustCompile(`(?is)href="(/eventInfo/\d+\.html)"`)
	homeRe := regexp.MustCompile(`(?is)<span class="home">.*?<span class="name">([^<]+)</span>`)
	awayRe := regexp.MustCompile(`(?is)<span class="away[^>]*>.*?<span class="name">([^<]+)</span>`)
	leagueRe := regexp.MustCompile(`(?is)<span class="title">([^<]+)</span>`)

	cands := make([]matchCandidate, 0, 64)
	for _, tag := range tagRe.FindAllString(html, -1) {
		hm := hrefRe.FindStringSubmatch(tag)
		tm := timeRe.FindStringSubmatch(tag)
		if len(hm) != 2 || len(tm) != 3 {
			continue
		}

		dateStr := tm[1] // 例如 "06-09"
		timeStr := tm[2] // 例如 "23:30"
		year := now.Year()
		fullTimeStr := fmt.Sprintf("%d-%s %s:00", year, dateStr, timeStr)
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", fullTimeStr, loc)
		
		// 跨年边界处理 (例如 12 月抓到了 1 月的比赛)
		if err == nil {
			if ts.After(now.Add(30 * 24 * time.Hour)) {
				ts = ts.AddDate(-1, 0, 0)
			} else if ts.Before(now.Add(-30 * 24 * time.Hour)) {
				ts = ts.AddDate(1, 0, 0)
			}
		}

		if err != nil || ts.Before(winStart) || ts.After(now) {
			continue
		}

		home := "未知主队"
		away := "未知客队"
		league := "未知联赛"

		if m := homeRe.FindStringSubmatch(tag); len(m) == 2 {
			home = strings.TrimSpace(m[1])
		}
		if m := awayRe.FindStringSubmatch(tag); len(m) == 2 {
			away = strings.TrimSpace(m[1])
		}
		if m := leagueRe.FindStringSubmatch(tag); len(m) == 2 {
			league = strings.TrimSpace(m[1])
		}

		// 清理无用空格与 HTML 实体
		home = strings.ReplaceAll(home, "&nbsp;", "")
		away = strings.ReplaceAll(away, "&nbsp;", "")
		league = strings.ReplaceAll(league, "&nbsp;", "")

		league, isHit := shortenLeagueName(league)

		var title string
		if isHit {
			league = strings.ReplaceAll(league, "-", "")
			title = fmt.Sprintf("%s:%svs%s", league, home, away)
		} else {
			title = fmt.Sprintf("%svs%s", home, away)
		}

		cands = append(cands, matchCandidate{Title: title, URL: hm[1], Time: timeStr, Timestamp: ts})
	}
	return cands
}

func extractM3U8(html string) string {
	re := regexp.MustCompile(`(?i)src=["']\s*([^"']+\.m3u8[^"']*)\s*["']`)
	m := re.FindStringSubmatch(html)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func normalizeStreamURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Path == "" {
		return ""
	}
	clean := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	if strings.HasPrefix(clean, "://") {
		clean = "https" + clean
	}
	return strings.ReplaceAll(clean, "adaptive", "1080p")
}

func writeOutputs(items []item, today string) error {
	var m3u strings.Builder
	var txt strings.Builder
	m3u.WriteString("#EXTM3U\n")
	m3u.WriteString("# DATE: " + today + "\n")

	if len(items) == 0 {
		m3u.WriteString("#EXTINF:-1 group-title=\"提示\", [00:00] 当前时段暂无符合条件的比赛\nhttp://127.0.0.1/empty.m3u8\n")
		txt.WriteString("当前时段暂无符合条件的比赛\n")
	} else {
		for _, it := range items {
			m3u.WriteString(fmt.Sprintf("#EXTINF:-1 group-title=\"直连线路\", [%s] %s\n", it.Time, it.Title))
			m3u.WriteString(it.URL + "\n")
			txt.WriteString(fmt.Sprintf("[直连线路] %s : %s\n", it.Title, it.URL))
		}
	}

	if err := os.WriteFile(playlistFile, []byte(m3u.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(liveLinksFile, []byte(txt.String()), 0o644); err != nil {
		return err
	}
	return nil
}
