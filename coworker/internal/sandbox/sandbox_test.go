package sandbox

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestSandbox_ToReal(t *testing.T) {
	// 使用跨平台路径
	realBase := filepath.Join("app", "userdata", "user123", "workspace")
	sb := NewSandbox("user123", realBase)

	tests := []struct {
		name        string
		virtualPath string
		wantRel     string // 相对于 realBase 的路径
		wantErr     bool
	}{
		{
			name:        "虚拟绝对路径",
			virtualPath: "/workspace/src/main.go",
			wantRel:     filepath.Join("src", "main.go"),
			wantErr:     false,
		},
		{
			name:        "虚拟根路径",
			virtualPath: "/workspace",
			wantRel:     "",
			wantErr:     false,
		},
		{
			name:        "相对路径",
			virtualPath: "src/main.go",
			wantRel:     filepath.Join("src", "main.go"),
			wantErr:     false,
		},
		{
			name:        "系统路径 - 应该报错",
			virtualPath: "/etc/passwd",
			wantRel:     "",
			wantErr:     true,
		},
		{
			name:        "路径遍历 - 应该报错",
			virtualPath: "../../../etc/passwd",
			wantRel:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sb.ToReal(tt.virtualPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToReal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				var wantReal string
				if tt.wantRel == "" {
					wantReal = realBase
				} else {
					wantReal = filepath.Join(realBase, tt.wantRel)
				}
				if got != wantReal {
					t.Errorf("ToReal() = %v, want %v", got, wantReal)
				}
			}
		})
	}
}

func TestSandbox_ToVirtual(t *testing.T) {
	realBase := filepath.Join("app", "userdata", "user123", "workspace")
	sb := NewSandbox("user123", realBase)

	tests := []struct {
		name     string
		realPath string
		want     string
	}{
		{
			name:     "沙箱内路径",
			realPath: filepath.Join(realBase, "src", "main.go"),
			want:     "/workspace/src/main.go",
		},
		{
			name:     "沙箱根路径",
			realPath: realBase,
			want:     "/workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sb.ToVirtual(tt.realPath); got != tt.want {
				t.Errorf("ToVirtual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandbox_VirtualizePaths(t *testing.T) {
	realBase := filepath.Join("app", "userdata", "user123", "workspace")
	sb := NewSandbox("user123", realBase)

	realPaths := []string{
		filepath.Join(realBase, "file1.go"),
		filepath.Join(realBase, "dir", "file2.go"),
	}

	want := []string{
		"/workspace/file1.go",
		"/workspace/dir/file2.go",
	}

	got := sb.VirtualizePaths(realPaths)

	if len(got) != len(want) {
		t.Errorf("VirtualizePaths() length = %v, want %v", len(got), len(want))
		return
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("VirtualizePaths()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestSandbox_VirtualizeOutput(t *testing.T) {
	realBase := filepath.Join("app", "userdata", "user123", "workspace")
	sb := NewSandbox("user123", realBase)

	input := "Error in " + realBase + "/src/main.go:10"
	want := "Error in /workspace/src/main.go:10"

	got := sb.VirtualizeOutput(input)
	if got != want {
		t.Errorf("VirtualizeOutput() = %v, want %v", got, want)
	}
}

func TestContainsTraversal(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"src/main.go", false},
		{"./src/main.go", false},
		{"../etc/passwd", true},
		{"src/../main.go", false},
		{"src/../../etc/passwd", true},
		{"/workspace/../../../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := containsTraversal(tt.path); got != tt.want {
				t.Errorf("containsTraversal(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// 跳过 Windows 上的某些测试
func skipOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}
}
