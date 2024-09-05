package main

import (
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

// IdiomsSolitaire 实现成语接龙
type IdiomsSolitaire struct {
	dict   map[rune][]string // "first_word" -> word_list
	idioms map[string]rune

	expiredTime int64 // 多长时间过期，单位秒
	maxTurn     int   // 单次会话持续多少轮
	maxMiss     int   // 每一轮最大失败多少次
	sessions    *xsync.MapOf[string, *Session]
}

type Idiom struct {
	Word      string `json:"word"`
	Pinyin    string `json:"pinyin"`
	PingyinR  string `json:"pinyin_r"`
	First     string `json:"first"`
	Last      string `json:"last"`
	FirstRune rune   `json:"first_rune"`
	LastRune  rune   `json:"last_rune"`
}

func NewIdiomsSolitaire(idioms []Idiom, expiredTime int64) *IdiomsSolitaire {
	var set = make(map[string]rune)
	var dict = make(map[rune][]string)
	for i := range idioms {
		idiom := &idioms[i]
		set[idiom.Word] = idiom.LastRune
		dict[idiom.FirstRune] = append(dict[idiom.FirstRune], idiom.Word)
	}

	is := &IdiomsSolitaire{}
	is.dict = dict
	is.idioms = set
	is.expiredTime = expiredTime
	is.sessions = xsync.NewMapOf[string, *Session]()

	is.maxTurn = 5
	is.maxMiss = 3

	return is
}

// radomIdiom 随机获取一个能够被接上的成语
func (is *IdiomsSolitaire) radomIdiom() (string, rune) {
	for word, last := range is.idioms {
		if _, ok := is.dict[last]; ok {
			return word, last
		}
	}
	log.Println("random failed")
	return "", 0
}

// Session 获取会话
func (is *IdiomsSolitaire) SessionOrCreate(key string) (*Session, bool) {
	// 随机选取一个成语
	ss := &Session{}
	ss.key = key
	ss.current, ss.lastRune = is.radomIdiom()
	ss.lastAccess = time.Now().Unix()

	var loaded bool
	ss, loaded = is.sessions.LoadOrStore(key, ss)

	return ss, loaded
}

// Session 获取会话
func (is *IdiomsSolitaire) Session(key string) *Session {
	ss, _ := is.sessions.Load(key)
	return ss
}

func (is *IdiomsSolitaire) SessionAndDelete(key string) (*Session, bool) {
	ss, exists := is.sessions.LoadAndDelete(key)
	// 修改会话状态
	if ss != nil {
		ss.Lock()
		defer ss.Unlock()
		ss.finished = true
	}
	return ss, exists
}

// ClearExpiredSession 清理过期的session
func (is *IdiomsSolitaire) ClearExpiredSession() {
	now := time.Now().Unix()
	is.sessions.Range(func(key string, value *Session) bool {
		if now-atomic.LoadInt64(&value.lastAccess) >= is.expiredTime {
			is.sessions.Delete(key)
		}
		return true
	})
}

// 1. 给到一个字符串，判断其是否为成语
// 2. 给到一个字符串，根据其首字/尾字分别取对应尾字/首字的成语

const (
	SolitaireFailed       = 1 + iota // 1. 接龙失败，返回提示
	SolitaireFailedToNext            // 2. 多次接龙失败，直接进入到下一轮，返回新的词
	SolitaireSucceed                 // 3. 接龙成功，进入下一轮，返回新的词
	SolitaireEnd                     // 4. 接龙成功，接不下去了，直接结束，返回结算数据
	SolitaireCompleted               // 5. 接龙完成，会话结束，返回结算数据
	SolitaireTimeout                 // 6. 会话已过期，
	SolitaireCanceled                // 7. 会话被退出了
	SolitaireFailComplete            // 8. 接龙完成，但是最后一轮失败
)

func (is *IdiomsSolitaire) Solitaire(ss *Session, idiom, id string) (int, string) {
	ss.Lock()
	defer ss.Unlock()

	// 0. 会话已结束
	if ss.finished {
		return SolitaireCanceled, ""
	}
	now := time.Now().Unix()
	if now-ss.lastAccess >= is.expiredTime {
		ss.finished = true // 会话已经超时，直接退出
		is.sessions.Delete(ss.key)
		return SolitaireTimeout, ""
	}
	atomic.StoreInt64(&ss.lastAccess, now)

	// 1. 检查是否在成语库中
	first := []rune(idiom)[0]
	_, ok := is.idioms[idiom]
	if !ok || first != ss.lastRune { // 不是成语，或不匹配
		log.Println("solitaire", ok, first, ss.lastRune)
		ss.miss += 1
		if is.maxMiss > 0 && ss.miss >= is.maxMiss {

			// 超出最大失败次数，给出答案，并进入到下一轮
			next, last, ok := is.nextIdiom(ss.current)
			if !ok { // 找不到下一个成语，提前结束
				return 0, ""
			}
			ss.trun += 1

			ss.miss = 0
			ss.current = next
			ss.lastRune = last

			if ss.trun >= is.maxTurn { // 接龙完成
				ss.finished = true
				is.sessions.Delete(ss.key)
				return SolitaireFailComplete, next
			}

			return SolitaireFailedToNext, next
		}
		return SolitaireFailed, ""
	}

	ss.trun += 1
	// 接龙成功计数+1
	if ss.hits == nil {
		ss.hits = map[string]int{}
	}
	ss.hits[id] = ss.hits[id] + 1

	// 3. 检查游戏轮数是否已经完成
	if ss.trun >= is.maxTurn { // 接龙完成
		ss.finished = true
		is.sessions.Delete(ss.key)
		return SolitaireCompleted, ""
	}

	// 4. 找到下一个可被接龙成语
	next, last, ok := is.nextIdiom(idiom)
	if !ok { // 找不到下一个成语，提前结束
		ss.finished = true
		is.sessions.Delete(ss.key)
		return SolitaireEnd, ""
	}

	// 返回新的成语
	ss.current = next
	ss.lastRune = last

	return SolitaireSucceed, next
}

// func (is *IdiomsSolitaire) isNextIdiom(current, next string) bool {

// }

// nextIdiom 找到可以被接龙的下一个成语（并且不等同于当前词），返回false说明无法再往下接
func (is *IdiomsSolitaire) nextIdiom(idiom string) (string, rune, bool) {
	// 检查是否在成语库
	last, ok := is.idioms[idiom]
	if !ok {
		return "", 0, false
	}
	// 根据最后一个字，取对应的成语列表
	list, ok := is.dict[last]
	if !ok {
		return "", 0, false
	}

	// 遍历列表，找到一个可以被接龙的成语
	for i := range list {
		next := list[i]
		if next == idiom {
			continue
		}
		last := is.idioms[next]
		if _, ok := is.dict[last]; !ok {
			continue
		}
		return list[i], last, true
	}
	return "", 0, false
}

func (is *IdiomsSolitaire) isValidIdiom(idiom string) bool {
	_, ok := is.idioms[idiom]
	return ok
}

type Session struct {
	key      string
	current  string         // 当前的成语
	lastRune rune           //
	miss     int            //  接龙失败了几次
	hits     map[string]int // 接龙成功的情况

	trun int //

	lastAccess int64 // 上次访问的时间

	sync.Mutex

	finished bool
}

func (s *Session) Idiom() string {
	return s.current
}

type HitCnt struct {
	Name  string
	Count int
}

type HitCntList []HitCnt

func (hc HitCntList) Len() int {
	return len(hc)
}
func (hc HitCntList) Less(i, j int) bool {
	return hc[i].Count < hc[j].Count
}

func (hc HitCntList) Swap(i, j int) {
	temp := hc[i]
	hc[i] = hc[j]
	hc[j] = temp
}

func (s *Session) Hits() []HitCnt {
	s.Lock()
	defer s.Unlock()
	var hits = make([]HitCnt, 0, len(s.hits))
	for k, v := range s.hits {
		hits = append(hits, HitCnt{k, v})
	}

	sort.Sort(HitCntList(hits))

	return hits
}
