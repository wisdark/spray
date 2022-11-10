package pkg

import (
	"encoding/json"
	"github.com/chainreactors/parsers"
	"github.com/chainreactors/spray/pkg/ihttp"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func NewBaseline(u, host string, resp *ihttp.Response) *Baseline {
	bl := &Baseline{
		Url:     u,
		Status:  resp.StatusCode(),
		IsValid: true,
	}
	uu, err := url.Parse(u)
	if err == nil {
		bl.Path = uu.Path
	}
	if resp.ClientType == ihttp.STANDARD {
		bl.Host = host
	}

	bl.Body = resp.Body()
	bl.BodyLength = resp.ContentLength()
	bl.Header = resp.Header()
	bl.HeaderLength = len(bl.Header)
	bl.RedirectURL = resp.GetHeader("Location")
	bl.Raw = append(bl.Header, bl.Body...)
	return bl
}

func NewInvalidBaseline(u, host string, resp *ihttp.Response, reason string) *Baseline {
	bl := &Baseline{
		Url:     u,
		Status:  resp.StatusCode(),
		IsValid: false,
		Reason:  reason,
	}

	uu, err := url.Parse(u)
	if err == nil {
		bl.Path = uu.Path
	}

	if resp.ClientType == ihttp.STANDARD {
		bl.Host = host
	}

	bl.Body = resp.Body()
	bl.BodyLength = resp.ContentLength()
	bl.RedirectURL = string(resp.GetHeader("Location"))

	return bl
}

type Baseline struct {
	Url          string     `json:"url"`
	Path         string     `json:"path"`
	Host         string     `json:"host"`
	Body         []byte     `json:"-"`
	BodyLength   int        `json:"body_length"`
	Header       []byte     `json:"-"`
	Raw          []byte     `json:"-"`
	HeaderLength int        `json:"header_length"`
	RedirectURL  string     `json:"redirect_url"`
	Status       int        `json:"status"`
	IsDynamicUrl bool       `json:"is_dynamic_url"` // 判断是否存在动态的url
	Spended      int        `json:"spended"`        // 耗时, 毫秒
	Title        string     `json:"title"`
	Frameworks   Frameworks `json:"frameworks"`
	Extracteds   Extracteds `json:"extracts"`
	Err          string     `json:"error"`
	Reason       string     `json:"reason"`
	IsValid      bool       `json:"valid"`
	IsFuzzy      bool       `json:"fuzzy"`
	*parsers.Hashes
}

// Collect 深度收集信息
func (bl *Baseline) Collect() {
	if len(bl.Body) > 0 {
		bl.Title = parsers.MatchTitle(string(bl.Body))
	}
	bl.Hashes = parsers.NewHashes(bl.Raw)
	// todo extract
	bl.Extracteds = Extractors.Extract(string(bl.Raw))
	bl.Frameworks = FingerDetect(string(bl.Raw))
}

// Compare
// if totally equal return 1
// if maybe equal return 0
// not equal return -1
func (bl *Baseline) Compare(other *Baseline) int {
	if other.RedirectURL != "" && bl.RedirectURL == other.RedirectURL {
		// 如果重定向url不为空, 且与base不相同, 则说明不是同一个页面
		return 1
	}

	if math.Abs(float64(bl.BodyLength-other.BodyLength)) < 16 {
		// 如果body length相等且md5相等, 则说明是同一个页面
		if bl.BodyMd5 == parsers.Md5Hash(other.Body) {
			// 如果length相等, md5也相等, 则判断为全同
			return 1
		} else {
			// 如果长度相等, 但是md5不相等, 可能是存在csrftoken之类的随机值
			return 0
		}
	} else {
		if strings.Contains(string(other.Body), other.Path) {
			// 如果包含路径本身, 可能是路径自身的随机值影响结果
			return 0
		} else {
			return -1
		}
	}
	return -1
}

var Distance uint8

func (bl *Baseline) FuzzyCompare(other *Baseline) bool {
	// todo 模糊匹配
	if parsers.SimhashCompare(other.BodySimhash, bl.BodySimhash) < Distance {
		return true
	}
	return false
}

func (bl *Baseline) Get(key string) string {
	switch key {
	case "url":
		return bl.Url
	case "host":
		return bl.Host
	case "title":
		return bl.Title
	case "redirect":
		return bl.RedirectURL
	case "md5":
		if bl.Hashes != nil {
			return bl.Hashes.BodyMd5
		} else {
			return ""
		}
	case "simhash":
		if bl.Hashes != nil {
			return bl.Hashes.BodySimhash
		} else {
			return ""
		}
	case "mmh3":
		if bl.Hashes != nil {
			return bl.Hashes.BodySimhash
		} else {
			return ""
		}
	case "stat", "status":
		return strconv.Itoa(bl.Status)
	case "spend":
		return strconv.Itoa(bl.Spended)
	//case "extract":
	//	return bl.Extracteds
	case "frame", "framework":
		return bl.Frameworks.ToString()
	default:
		return ""
	}
}

func (bl *Baseline) Additional(key string) string {
	if v := bl.Get(key); v != "" {
		return " [" + v + "] "
	} else {
		return " "
	}
}

func (bl *Baseline) Format(probes []string) string {
	var line strings.Builder
	line.WriteString(bl.Url)
	if bl.Host != "" {
		line.WriteString(" (" + bl.Host + ")")
	}

	if bl.Reason != "" {
		line.WriteString(" ,")
		line.WriteString(bl.Reason)
	}
	if bl.Err != "" {
		line.WriteString(" ,err: ")
		line.WriteString(bl.Err)
		return line.String()
	}

	for _, p := range probes {
		line.WriteString(" ")
		line.WriteString(bl.Additional(p))
	}

	return line.String()
}

func (bl *Baseline) String() string {
	var line strings.Builder
	//line.WriteString("[+] ")
	line.WriteString(bl.Url)
	if bl.Host != "" {
		line.WriteString(" (" + bl.Host + ")")
	}

	if bl.Err != "" {
		line.WriteString("err: ")
		line.WriteString(bl.Err)
		return line.String()
	}

	line.WriteString(" - ")
	line.WriteString(strconv.Itoa(bl.Status))
	line.WriteString(" - ")
	line.WriteString(strconv.Itoa(bl.BodyLength))
	if bl.RedirectURL != "" {
		line.WriteString(" -> ")
		line.WriteString(bl.RedirectURL)
		line.WriteString(" ")
	}
	line.WriteString(bl.Additional("title"))
	line.WriteString(bl.Frameworks.ToString())

	return line.String()
}

func (bl *Baseline) Jsonify() string {
	bs, err := json.Marshal(bl)
	if err != nil {
		return ""
	}
	return string(bs)
}