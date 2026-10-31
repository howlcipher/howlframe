package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

// The web_app archive and download primitives are browser host operations:
// check enforces their arity, and targets without a browser host refuse them.
func TestBrowserArchivePrimitivesCheckAndTargetBoundaries(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	for _, tc := range []struct{ name, src, want string }{
		{"zip_build no args", `(web_app (zip_build))`, "zip_build expects"},
		{"zip_build two args", `(web_app (zip_build (list) (list)))`, "zip_build expects"},
		{"download_file arity", `(web_app (download_file "a" "b"))`, "download_file expects"},
		{"dom_text arity", `(web_app (dom_text))`, "dom_text expects"},
		{"cli zip_build", `(cli_app (print (zip_build (list))))`, "zip_build"},
		{"cli download_file", `(cli_app (download_file "a" "b" "AA=="))`, "download_file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := mustWriteFile(t, dir, strings.ReplaceAll(tc.name, " ", "_")+".howl", tc.src)
			out, err := exec.Command(bin, "build", path, "-o", filepath.Join(dir, "out.hfbc")).CombinedOutput()
			if strings.HasPrefix(tc.src, "(web_app") {
				out, err = exec.Command(bin, "check", path).CombinedOutput()
			}
			if err == nil {
				t.Fatalf("%s must be rejected, got success: %s", tc.src, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("want diagnostic containing %q, got %s", tc.want, out)
			}
		})
	}
}

func TestBrowserDomHostOperationArity(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	for _, src := range []string{
		`(web_app (let (e "x") (event_attr e)))`, `(web_app (event_key))`, `(web_app (event_prevent))`, `(web_app (set_value (dom_query "#a")))`,
		`(web_app (dom_checked))`, `(web_app (set_checked (dom_query "#a")))`, `(web_app (dom_focus))`, `(web_app (remove_attr (dom_query "#a")))`,
		`(web_app (dialog_open))`, `(web_app (dialog_close))`,
		`(web_app (storage_get "a"))`, `(web_app (storage_put "a" "b"))`, `(web_app (storage_keys))`, `(web_app (file_read_input (dom_query "#f")))`,
		`(web_app (richtext_get))`, `(web_app (richtext_set (dom_query "#e")))`, `(web_app (richtext_command (dom_query "#e") "bold"))`, `(web_app (richtext_attach (dom_query "#e")))`,
		`(web_app (sniff_media_type))`, `(web_app (image_decode "x"))`, `(web_app (image_encode "x" "image/png" 10 "image/png"))`,
	} {
		path := mustWriteFile(t, dir, "a.howl", src)
		out, err := exec.Command(bin, "check", path).CombinedOutput()
		if err == nil || !strings.Contains(string(out), " expects (") {
			t.Fatalf("%s: want an arity diagnostic, got %v %s", src, err, out)
		}
	}
}

// Every browser host operation in capability.HostOperations has its arity
// checked against the table, and a lambda parameter must be a lambda form.
func TestEveryBrowserHostOperationArity(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	for _, op := range capability.HostOperations() {
		args := []string{op.Name}
		for _, p := range op.Params {
			if p.Kind == capability.HostLambda {
				args = append(args, "(lambda ("+strings.Join(p.Lambda, " ")+") (print 1))")
			} else {
				args = append(args, `"x"`)
			}
		}
		cases := map[string]string{
			"(web_app (" + strings.Join(append(args, `"extra"`), " ") + "))": op.Name + " expects " + op.Usage(),
		}
		if op.LambdaLast() {
			cases["(web_app ("+strings.Join(append(args[:len(args)-1:len(args)-1], "5"), " ")+"))"] = op.Name + " expects a lambda as its last argument"
		}
		for src, want := range cases {
			path := mustWriteFile(t, dir, "a.howl", src)
			out, err := exec.Command(bin, "check", path).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s: want %q, got %v %s", src, want, err, out)
			}
		}
	}
}
