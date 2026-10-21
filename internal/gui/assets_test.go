package gui

import (
	"strings"
	"testing"
)

func TestAssetsHTMLInlinesEverything(t *testing.T) {
	html := assetsHTML()
	if strings.Contains(html, "{{CSS}}") || strings.Contains(html, "{{JS}}") {
		t.Fatal("placeholders must be replaced")
	}
	if !strings.Contains(html, "开机启动管理器") {
		t.Fatal("title missing")
	}
	if !strings.Contains(html, "window.loadApp") {
		t.Fatal("script missing")
	}
	if !strings.Contains(html, "--accent") {
		t.Fatal("stylesheet missing")
	}
	if !strings.HasPrefix(html, "<!doctype html>") {
		t.Fatal("unexpected document start")
	}
}
