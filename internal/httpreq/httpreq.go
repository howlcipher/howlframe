// Package httpreq is the shared contract for reading an already-accepted
// HTTP request. The bytecode VM, the tree-walking interpreter, and the Go
// backend all call these functions so a missing name, a wrong receiver, and
// a path-segment capture cannot drift between them.
//
// The reads are pure. They do not open a socket and they do not grant or
// require a capability. Accepting the request still requires network,
// through the HTTP server opcodes that already exist.
package httpreq

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
)

// Error is a fail-closed request-surface failure. Code is a HowlFrame
// runtime code such as TYPE_ERROR or RUNTIME_ERROR. Message does not repeat
// that code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

func typeError(format string, args ...any) error {
	return &Error{Code: "TYPE_ERROR", Message: fmt.Sprintf(format, args...)}
}

func runtimeError(format string, args ...any) error {
	return &Error{Code: "RUNTIME_ERROR", Message: fmt.Sprintf(format, args...)}
}

type paramsKey struct{}

// WithParams returns a request whose path captures are visible to Path.
// The map is copied. A nil map records no captures, which Path reads as a
// miss.
func WithParams(r *http.Request, params map[string]string) *http.Request {
	if r == nil {
		return nil
	}
	clone := make(map[string]string, len(params))
	for key, value := range params {
		clone[key] = value
	}
	return r.WithContext(context.WithValue(r.Context(), paramsKey{}, clone))
}

// Query returns the first query-string value for name. A missing name and a
// present-but-empty value are both "". The name is case-sensitive. Repeated
// keys contribute only the first value.
func Query(recv, name any) (string, error) {
	req, err := readRequest("req_query", recv)
	if err != nil {
		return "", err
	}
	key, err := readName("req_query", name)
	if err != nil {
		return "", err
	}
	if req.URL == nil {
		return "", typeError("req_query expected *http.Request with a URL")
	}
	return req.URL.Query().Get(key), nil
}

// Header returns the first value of a request header. Lookup is
// case-insensitive. A missing header is "". Repeated values contribute only
// the first one.
func Header(recv, name any) (string, error) {
	req, err := readRequest("req_header", recv)
	if err != nil {
		return "", err
	}
	key, err := readName("req_header", name)
	if err != nil {
		return "", err
	}
	return req.Header.Get(key), nil
}

// Path returns one named path capture previously attached to the request.
// A name that was not captured is "". Names are case-sensitive.
func Path(recv, name any) (string, error) {
	req, err := readRequest("req_path", recv)
	if err != nil {
		return "", err
	}
	key, err := readName("req_path", name)
	if err != nil {
		return "", err
	}
	raw := req.Context().Value(paramsKey{})
	if raw == nil {
		return "", nil
	}
	params, ok := raw.(map[string]string)
	if !ok {
		return "", typeError("req_path expected path parameters on the request")
	}
	value, ok := params[key]
	if !ok {
		return "", nil
	}
	return value, nil
}

func readRequest(op string, recv any) (*http.Request, error) {
	req, ok := recv.(*http.Request)
	if !ok || req == nil {
		return nil, typeError("%s expected *http.Request, got %T", op, recv)
	}
	return req, nil
}

func readName(op string, raw any) (string, error) {
	name, ok := raw.(string)
	if !ok {
		return "", typeError("%s expected non-empty string name, got %T", op, raw)
	}
	if name == "" {
		return "", typeError("%s expected non-empty string name", op)
	}
	return name, nil
}

type segment struct {
	literal string
	param   string
}

// Pattern is a route path. Parametric is true when at least one segment is
// a {name} capture. A path with no braces is a literal route.
type Pattern struct {
	Raw        string
	Parametric bool
	clean      string
	segments   []segment
}

// ParseRoute accepts a literal path or a path whose dynamic pieces are whole
// segments of the form {name}. name is an identifier. Braces that do not
// wrap a whole segment, duplicate names, and an empty path fail closed.
func ParseRoute(path string) (Pattern, error) {
	if path == "" {
		return Pattern{}, typeError("route path must not be empty")
	}
	cleaned := cleanRoutePath(path)
	parts := strings.Split(cleaned, "/")
	segs := make([]segment, len(parts))
	seen := make(map[string]bool, len(parts))
	parametric := false
	for i, part := range parts {
		if !strings.Contains(part, "{") && !strings.Contains(part, "}") {
			segs[i] = segment{literal: part}
			continue
		}
		name, ok := captureName(part)
		if !ok || !validParamName(name) {
			return Pattern{}, typeError("invalid route pattern %q: %q must be a {name} segment", path, part)
		}
		if seen[name] {
			return Pattern{}, typeError("invalid route pattern %q: duplicate parameter %q", path, name)
		}
		seen[name] = true
		parametric = true
		segs[i] = segment{param: name}
	}
	return Pattern{Raw: path, Parametric: parametric, clean: cleaned, segments: segs}, nil
}

func captureName(part string) (string, bool) {
	if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' {
		return "", false
	}
	name := part[1 : len(part)-1]
	if strings.ContainsAny(name, "{}") {
		return "", false
	}
	return name, true
}

func validParamName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// Match reports whether requestPath captures this pattern. The path is
// cleaned first, including "." and ".." segments, and a trailing slash is
// preserved. An empty capture does not match. Literal patterns do not match
// here; the server compares those by exact cleaned path.
func (p Pattern) Match(requestPath string) (map[string]string, bool) {
	if !p.Parametric {
		return nil, false
	}
	parts := strings.Split(cleanRoutePath(requestPath), "/")
	if len(parts) != len(p.segments) {
		return nil, false
	}
	params := make(map[string]string)
	for i, seg := range p.segments {
		if seg.param != "" {
			if parts[i] == "" {
				return nil, false
			}
			params[seg.param] = parts[i]
			continue
		}
		if parts[i] != seg.literal {
			return nil, false
		}
	}
	return params, true
}

func (p Pattern) skeleton() string {
	parts := make([]string, len(p.segments))
	for i, seg := range p.segments {
		if seg.param != "" {
			parts[i] = "{}"
		} else {
			parts[i] = seg.literal
		}
	}
	return strings.Join(parts, "/")
}

// cleanRoutePath matches net/http's path cleaning closely enough for route
// matching: slash-separated, "." and ".." removed, and a trailing slash kept
// when the caller wrote one.
func cleanRoutePath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	trailing := len(p) > 1 && strings.HasSuffix(p, "/")
	cleaned := path.Clean(p)
	if trailing && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

type registered struct {
	pattern Pattern
	handler http.HandlerFunc
}

// Server dispatches literal routes and whole-segment {name} patterns.
// Literal paths stay registered on the inner ServeMux so existing mux tests
// keep seeing them. Pattern routes are not written onto that mux, because a
// Go 1.21 ServeMux would treat "{id}" as literal text.
type Server struct {
	mux       *http.ServeMux
	literals  map[string]http.HandlerFunc
	patterns  []registered
	skeletons map[string]struct{}
}

// NewServer wraps mux. A nil mux gets a fresh ServeMux.
func NewServer(mux *http.ServeMux) *Server {
	if mux == nil {
		mux = http.NewServeMux()
	}
	return &Server{
		mux:       mux,
		literals:  make(map[string]http.HandlerFunc),
		skeletons: make(map[string]struct{}),
	}
}

// Handle registers path. An exact literal wins over a pattern with the same
// cleaned path. A second pattern with the same literal skeleton (parameter
// names ignored) is a duplicate. Malformed braces fail closed.
func (s *Server) Handle(routePath string, handler http.HandlerFunc) error {
	if s == nil {
		return runtimeError("http server is not started")
	}
	if handler == nil {
		return runtimeError("route handler is missing")
	}
	pattern, err := ParseRoute(routePath)
	if err != nil {
		return err
	}
	if !pattern.Parametric {
		if _, exists := s.literals[pattern.clean]; exists {
			return runtimeError("duplicate route %q", routePath)
		}
		s.literals[pattern.clean] = handler
		s.mux.HandleFunc(routePath, handler)
		return nil
	}
	skeleton := pattern.skeleton()
	if _, exists := s.skeletons[skeleton]; exists {
		return runtimeError("duplicate path pattern %q", routePath)
	}
	s.skeletons[skeleton] = struct{}{}
	s.patterns = append(s.patterns, registered{pattern: pattern, handler: handler})
	return nil
}

// ServeHTTP prefers an exact literal, then the first matching {name}
// pattern, then the inner ServeMux (its 404 and any slash redirects).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s == nil || r == nil || r.URL == nil {
		http.NotFound(w, r)
		return
	}
	cleaned := cleanRoutePath(r.URL.Path)
	if handler, ok := s.literals[cleaned]; ok {
		handler(w, r)
		return
	}
	for _, reg := range s.patterns {
		params, ok := reg.pattern.Match(cleaned)
		if !ok {
			continue
		}
		reg.handler(w, WithParams(r, params))
		return
	}
	s.mux.ServeHTTP(w, r)
}
