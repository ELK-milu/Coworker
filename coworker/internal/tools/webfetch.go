package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/coworker/pkg/types"
	"golang.org/x/net/html"
)

const (
	webFetchTimeout  = 30 * time.Second
	webFetchMaxBytes = 1 << 20
)

// WebFetchTool fetches a web page and returns Markdown.
type WebFetchTool struct{}

type WebFetchInput struct {
	URL string `json:"url"`
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{}
}

func (t *WebFetchTool) Name() string { return "WebFetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch a web page via HTTP GET and return Markdown content."
}

func (t *WebFetchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{"type": "string"},
		},
		"required": []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*types.ToolResult, error) {
	var in WebFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	in.URL = strings.TrimSpace(in.URL)
	if in.URL == "" {
		return &types.ToolResult{Success: false, Error: "url is required"}, nil
	}

	parsed, err := url.Parse(in.URL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return &types.ToolResult{Success: false, Error: "invalid url"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, webFetchTimeout)
	defer cancel()

	if err := validateFetchURL(ctx, parsed); err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	client := &http.Client{
		Timeout: webFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateFetchURL(req.Context(), req.URL); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", "Coworker-WebFetch/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("http error: %s", resp.Status)}, nil
	}
	if resp.ContentLength > webFetchMaxBytes {
		return &types.ToolResult{Success: false, Error: "response exceeds 1MB limit"}, nil
	}

	body, err := readLimited(resp.Body, webFetchMaxBytes)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	var output string
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml") || contentType == "" {
		output = htmlToMarkdown(body, parsed)
	} else {
		output = strings.TrimSpace(string(body))
	}

	return &types.ToolResult{Success: true, Output: output}, nil
}

func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes limit", maxBytes)
	}
	return data, nil
}

func validateFetchURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return errors.New("invalid url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("missing host")
	}
	if isLocalHostname(host) {
		return errors.New("localhost is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return errors.New("private network is not allowed")
		}
		return nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		if isPrivateIP(addr.IP) {
			return errors.New("private network is not allowed")
		}
	}
	return nil
}

func isLocalHostname(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" {
		return true
	}
	return strings.HasSuffix(h, ".localhost")
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	return false
}

func htmlToMarkdown(input []byte, baseURL *url.URL) string {
	doc, err := html.Parse(bytes.NewReader(input))
	if err != nil {
		return strings.TrimSpace(string(input))
	}

	root := findBodyNode(doc)
	if root == nil {
		root = doc
	}

	renderer := &mdRenderer{baseURL: baseURL}
	renderer.render(root)
	return cleanupMarkdown(renderer.b.String())
}

func findBodyNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findBodyNode(c); found != nil {
			return found
		}
	}
	return nil
}

type mdRenderer struct {
	b         strings.Builder
	baseURL   *url.URL
	inPre     bool
	listStack []listContext
}

type listContext struct {
	ordered bool
	index   int
}

func (r *mdRenderer) render(n *html.Node) {
	switch n.Type {
	case html.DocumentNode:
		r.renderChildren(n)
	case html.TextNode:
		r.writeText(n.Data)
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		switch tag {
		case "script", "style", "noscript", "head":
			return
		case "br":
			r.b.WriteString("\n")
		case "hr":
			r.ensureBlock()
			r.b.WriteString("---")
			r.ensureBlock()
		case "h1", "h2", "h3", "h4", "h5", "h6":
			level := int(tag[1] - '0')
			if level < 1 || level > 6 {
				level = 2
			}
			r.ensureBlock()
			r.b.WriteString(strings.Repeat("#", level))
			r.b.WriteString(" ")
			r.renderInlineChildren(n)
			r.ensureBlock()
		case "p", "div", "section", "article", "header", "footer", "main", "nav", "aside":
			r.ensureBlock()
			r.renderInlineChildren(n)
			r.ensureBlock()
		case "strong", "b":
			r.b.WriteString("**")
			r.renderInlineChildren(n)
			r.b.WriteString("**")
		case "em", "i":
			r.b.WriteString("*")
			r.renderInlineChildren(n)
			r.b.WriteString("*")
		case "code":
			if r.inPre {
				r.renderChildren(n)
				return
			}
			r.b.WriteString("`")
			r.renderInlineChildren(n)
			r.b.WriteString("`")
		case "pre":
			r.ensureBlock()
			r.b.WriteString("```\n")
			prev := r.inPre
			r.inPre = true
			r.renderChildren(n)
			r.inPre = prev
			r.b.WriteString("\n```\n")
		case "a":
			r.renderLink(n)
		case "img":
			r.renderImage(n)
		case "ul", "ol":
			r.ensureBlock()
			r.pushList(tag == "ol")
			r.renderChildren(n)
			r.popList()
			r.ensureBlock()
		case "li":
			r.renderListItem(n)
		case "blockquote":
			r.ensureBlock()
			quote := cleanupMarkdown(r.renderChildrenToString(n))
			if quote != "" {
				lines := strings.Split(quote, "\n")
				for _, line := range lines {
					if line == "" {
						r.b.WriteString(">\n")
						continue
					}
					r.b.WriteString("> ")
					r.b.WriteString(line)
					r.b.WriteString("\n")
				}
			}
			r.ensureBlock()
		default:
			r.renderChildren(n)
		}
	}
}

func (r *mdRenderer) renderInline(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		r.writeText(n.Data)
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		switch tag {
		case "script", "style", "noscript", "head":
			return
		case "br":
			r.b.WriteString(" ")
		case "strong", "b":
			r.b.WriteString("**")
			r.renderInlineChildren(n)
			r.b.WriteString("**")
		case "em", "i":
			r.b.WriteString("*")
			r.renderInlineChildren(n)
			r.b.WriteString("*")
		case "code":
			if r.inPre {
				r.renderInlineChildren(n)
				return
			}
			r.b.WriteString("`")
			r.renderInlineChildren(n)
			r.b.WriteString("`")
		case "a":
			r.renderLink(n)
		case "img":
			r.renderImage(n)
		case "ul", "ol", "li":
			r.render(n)
		default:
			r.renderInlineChildren(n)
		}
	}
}

func (r *mdRenderer) renderChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.render(c)
	}
}

func (r *mdRenderer) renderInlineChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.renderInline(c)
	}
}

func (r *mdRenderer) renderChildrenToString(n *html.Node) string {
	temp := &mdRenderer{baseURL: r.baseURL}
	temp.renderChildren(n)
	return temp.b.String()
}

func (r *mdRenderer) renderInlineToString(n *html.Node) string {
	temp := &mdRenderer{baseURL: r.baseURL}
	temp.renderInlineChildren(n)
	return temp.b.String()
}

func (r *mdRenderer) renderLink(n *html.Node) {
	href := strings.TrimSpace(getAttr(n, "href"))
	text := strings.TrimSpace(cleanupInline(r.renderInlineToString(n)))
	if text == "" {
		text = href
	}
	if href == "" {
		r.b.WriteString(text)
		return
	}
	r.b.WriteString("[")
	r.b.WriteString(text)
	r.b.WriteString("](")
	r.b.WriteString(resolveURL(r.baseURL, href))
	r.b.WriteString(")")
}

func (r *mdRenderer) renderImage(n *html.Node) {
	src := strings.TrimSpace(getAttr(n, "src"))
	if src == "" {
		return
	}
	alt := strings.TrimSpace(getAttr(n, "alt"))
	r.b.WriteString("![")
	r.b.WriteString(alt)
	r.b.WriteString("](")
	r.b.WriteString(resolveURL(r.baseURL, src))
	r.b.WriteString(")")
}

func (r *mdRenderer) renderListItem(n *html.Node) {
	indent := strings.Repeat("  ", maxInt(0, len(r.listStack)-1))
	marker := "-"
	if len(r.listStack) > 0 && r.listStack[len(r.listStack)-1].ordered {
		r.listStack[len(r.listStack)-1].index++
		marker = fmt.Sprintf("%d.", r.listStack[len(r.listStack)-1].index)
	}
	r.b.WriteString("\n")
	r.b.WriteString(indent)
	r.b.WriteString(marker)
	r.b.WriteString(" ")

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (strings.EqualFold(c.Data, "ul") || strings.EqualFold(c.Data, "ol")) {
			r.render(c)
		} else {
			r.renderInline(c)
		}
	}
}

func (r *mdRenderer) writeText(text string) {
	if r.inPre {
		r.b.WriteString(text)
		return
	}
	normalized := strings.Join(strings.Fields(text), " ")
	if normalized == "" {
		return
	}
	if r.b.Len() > 0 {
		last := r.lastByte()
		if last != '\n' && last != ' ' {
			r.b.WriteByte(' ')
		}
	}
	r.b.WriteString(normalized)
}

func (r *mdRenderer) lastByte() byte {
	s := r.b.String()
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func (r *mdRenderer) ensureBlock() {
	if r.b.Len() == 0 {
		return
	}
	r.b.WriteString("\n\n")
}

func (r *mdRenderer) pushList(ordered bool) {
	r.listStack = append(r.listStack, listContext{ordered: ordered})
}

func (r *mdRenderer) popList() {
	if len(r.listStack) == 0 {
		return
	}
	r.listStack = r.listStack[:len(r.listStack)-1]
}

func getAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || base == nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}

func cleanupInline(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

func cleanupMarkdown(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text = strings.TrimSpace(strings.Join(lines, "\n"))
	re := regexp.MustCompile("\n{3,}")
	text = re.ReplaceAllString(text, "\n\n")
	return text
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
