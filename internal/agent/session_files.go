package agent

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func contained(parent, child string) bool {
	rel, e := filepath.Rel(parent, child)
	return e == nil && (rel == "." || filepath.IsLocal(rel))
}
func secureLocation(path string) (string, error) {
	if !utf8.ValidString(path) {
		return "", errors.New("session paths must be valid UTF-8")
	}
	absolute, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	if !utf8.ValidString(absolute) {
		return "", errors.New("absolute session paths must be valid UTF-8")
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(absolute, string(filepath.Separator)), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink path forbidden: %s", current)
		}
	}
	return absolute, nil
}
func secureDirectory(path string) (string, error) {
	p, e := secureLocation(path)
	if e != nil {
		return "", e
	}
	s, e := os.Stat(p)
	if e != nil {
		return "", e
	}
	if !s.IsDir() {
		return "", errors.New("expected directory")
	}
	return p, nil
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func sessionMAC(key, data []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return hex.EncodeToString(m.Sum(nil))
}
func readSessionFile(dir, name string, limit int64) ([]byte, error) {
	if !filepath.IsLocal(name) {
		return nil, errors.New("nonlocal path")
	}
	p, e := secureLocation(filepath.Join(dir, name))
	if e != nil {
		return nil, e
	}
	info, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("nonregular or oversized file")
	}
	root, e := os.OpenRoot(dir)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	f, e := root.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("file exceeds limit")
	}
	return b, e
}
func sessionKey(dir string, create bool) ([]byte, error) {
	if create {
		if e := os.Mkdir(dir, 0700); e != nil && !os.IsExist(e) {
			return nil, e
		}
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("trust directory must be private (0700)")
	}
	path := filepath.Join(dir, "key")
	if create {
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e == nil {
			key := make([]byte, 32)
			_, e = rand.Read(key)
			if e == nil {
				_, e = f.Write(key)
			}
			if e == nil {
				e = f.Sync()
			}
			closeErr := f.Close()
			if e != nil {
				return nil, e
			}
			if closeErr != nil {
				return nil, closeErr
			}
		} else if !os.IsExist(e) {
			return nil, e
		}
	}
	info, e = os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("trust key must be a private regular file")
	}
	key, e := readSessionFile(dir, "key", 32)
	if e != nil {
		return nil, e
	}
	if len(key) != 32 {
		return nil, errors.New("invalid trust key length")
	}
	return key, nil
}
func atomicSessionFile(dir, name string, data []byte) error {
	f, e := os.CreateTemp(dir, ".pending-")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if e = os.Rename(temp, filepath.Join(dir, name)); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func strictSessionJSON(data []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(value); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
