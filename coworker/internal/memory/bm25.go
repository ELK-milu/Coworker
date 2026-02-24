package memory

import (
	"math"
	"strings"
	"sync"
	"unicode"

	"github.com/wangbin/jiebago"
)

// BM25Ranker BM25 排序器
type BM25Ranker struct {
	K1 float64 // 词频饱和参数，默认 1.5
	B  float64 // 长度惩罚参数，默认 0.75
}

// NewBM25Ranker 创建 BM25 排序器
func NewBM25Ranker() *BM25Ranker {
	return &BM25Ranker{
		K1: 1.5,
		B:  0.75,
	}
}

// Score 计算 BM25 分数
func (r *BM25Ranker) Score(queryTokens, docTokens []string, avgDocLength float64, idfScores map[string]float64) float64 {
	docLength := float64(len(docTokens))

	// 统计词频
	termFreq := make(map[string]int)
	for _, token := range docTokens {
		termFreq[token]++
	}

	var score float64
	for _, token := range queryTokens {
		tf := float64(termFreq[token])
		if tf == 0 {
			continue
		}

		idf := idfScores[token]
		if idf == 0 {
			continue
		}

		// BM25 公式
		numerator := tf * (r.K1 + 1)
		denominator := tf + r.K1*(1-r.B+r.B*(docLength/avgDocLength))

		score += idf * (numerator / denominator)
	}

	return score
}

// CalculateIDF 计算 IDF（逆文档频率）
func (r *BM25Ranker) CalculateIDF(allDocs [][]string) map[string]float64 {
	N := float64(len(allDocs))
	df := make(map[string]int) // document frequency

	// 统计每个词出现在多少文档中
	for _, doc := range allDocs {
		uniqueTokens := make(map[string]bool)
		for _, token := range doc {
			uniqueTokens[token] = true
		}
		for token := range uniqueTokens {
			df[token]++
		}
	}

	// 计算 IDF
	idfScores := make(map[string]float64)
	for token, docFreq := range df {
		// IDF = log((N - df + 0.5) / (df + 0.5) + 1)
		dfFloat := float64(docFreq)
		idfScores[token] = math.Log((N-dfFloat+0.5)/(dfFloat+0.5) + 1)
	}

	return idfScores
}

// Tokenizer 分词器（使用 Jieba）
type Tokenizer struct {
	jieba     *jiebago.Segmenter
	stopWords map[string]bool
	initOnce  sync.Once
	initErr   error
}

// 全局分词器实例
var globalTokenizer *Tokenizer
var tokenizerOnce sync.Once

// NewTokenizer 创建分词器
func NewTokenizer() *Tokenizer {
	tokenizerOnce.Do(func() {
		globalTokenizer = &Tokenizer{
			stopWords: defaultStopWords(),
		}
		globalTokenizer.initJieba()
	})
	return globalTokenizer
}

// initJieba 初始化 Jieba 分词器
func (t *Tokenizer) initJieba() {
	t.initOnce.Do(func() {
		t.jieba = &jiebago.Segmenter{}
		// jiebago 会自动加载内置词典
		t.initErr = t.jieba.LoadDictionary("dict.txt")
		if t.initErr != nil {
			// 如果加载失败，使用空词典（降级到简单分词）
			t.initErr = nil
		}
	})
}

// defaultStopWords 默认停用词
func defaultStopWords() map[string]bool {
	return map[string]bool{
		// 中文停用词
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"你": true, "他": true, "她": true, "它": true, "这": true,
		"那": true, "有": true, "个": true, "就": true, "不": true,
		"人": true, "都": true, "一": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"能": true, "会": true, "和": true, "与": true, "或": true,
		"但": true, "如果": true, "那么": true, "这个": true, "那个": true,
		"我们": true, "你们": true, "他们": true, "什么": true, "怎么": true,
		"为什么": true, "哪里": true, "可以": true, "没有": true, "因为": true,
		"所以": true, "但是": true, "而且": true, "然后": true, "如何": true,
		// 英文停用词
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "this": true, "that": true, "what": true,
		"which": true, "who": true, "how": true, "why": true, "where": true,
		"when": true, "and": true, "or": true, "but": true, "if": true,
		"for": true, "of": true, "to": true, "from": true, "in": true,
		"on": true, "at": true, "by": true, "with": true, "about": true,
	}
}

// Tokenize 分词（使用 Jieba）
func (t *Tokenizer) Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string

	// 使用 Jieba 分词
	if t.jieba != nil {
		ch := t.jieba.Cut(text, true) // 精确模式
		for word := range ch {
			word = strings.ToLower(strings.TrimSpace(word))
			if len(word) >= 1 && !t.stopWords[word] && !isAllPunctuation(word) {
				tokens = append(tokens, word)
			}
		}
		return tokens
	}

	// 降级：简单分词
	return t.simpleSplit(text)
}

// isAllPunctuation 检查是否全是标点符号
func isAllPunctuation(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// simpleSplit 简单分词（降级方案）
func (t *Tokenizer) simpleSplit(text string) []string {
	var tokens []string
	var currentToken strings.Builder

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			if currentToken.Len() > 0 {
				word := strings.ToLower(currentToken.String())
				if !t.stopWords[word] && len(word) >= 1 {
					tokens = append(tokens, word)
				}
				currentToken.Reset()
			}
			char := string(r)
			if !t.stopWords[char] {
				tokens = append(tokens, char)
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentToken.WriteRune(r)
		} else {
			if currentToken.Len() > 0 {
				word := strings.ToLower(currentToken.String())
				if !t.stopWords[word] && len(word) >= 1 {
					tokens = append(tokens, word)
				}
				currentToken.Reset()
			}
		}
	}

	if currentToken.Len() > 0 {
		word := strings.ToLower(currentToken.String())
		if !t.stopWords[word] && len(word) >= 1 {
			tokens = append(tokens, word)
		}
	}

	return tokens
}
