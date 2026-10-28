package infra_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdAssetsContainRequiredSettings(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want []string
	}{
		{
			name: "server unit",
			path: []string{"systemd", "tes3mp.service"},
			want: []string{
				"User=tes3mp",
				"Group=tes3mp",
				"Environment=HOME=/tes3mp-easy",
				"WorkingDirectory=/tes3mp-easy",
				"ExecStart=/opt/tes3mp-easy/tes3mp/tes3mp-server",
				"Restart=on-failure",
				"RestartSec=5s",
				"MemoryHigh=384M",
				"MemoryMax=448M",
				"OOMScoreAdjust=-500",
				"UMask=0027",
				"ProtectSystem=strict",
				"PrivateTmp=yes",
				"ProtectHome=yes",
				"ReadWritePaths=/tes3mp-easy/players /tes3mp-easy/world /tes3mp-easy/configs",
				"WantedBy=multi-user.target",
			},
		},
		{
			name: "panel unit",
			path: []string{"systemd", "tes3mp-panel.service"},
			want: []string{
				"User=tes3mp-panel",
				"Group=tes3mp-panel",
				"SupplementaryGroups=tes3mp",
				"EnvironmentFile=-/etc/tes3mp-easy/panel.env",
				"ExecStart=/opt/tes3mp-easy/bin/tes3mp-easy-panel",
				"StartLimitBurst=2",
				"RestartSec=60s",
				"TimeoutStartSec=5min",
				"Environment=GOMAXPROCS=1",
				"Environment=GOMEMLIMIT=48MiB",
				"MemoryHigh=48M",
				"MemoryMax=64M",
				"OOMScoreAdjust=500",
				"AmbientCapabilities=CAP_NET_BIND_SERVICE",
				"CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID CAP_AUDIT_WRITE CAP_SYS_RESOURCE",
				"NoNewPrivileges=no",
				"ReadWritePaths=/tes3mp-easy",
			},
		},
		{
			name: "backup service",
			path: []string{"systemd", "tes3mp-easy-backup.service"},
			want: []string{
				"Type=oneshot",
				"User=tes3mp",
				"ConditionPathExists=/opt/tes3mp-easy/bin/tes3mp-easy-ops",
				"ExecStart=/opt/tes3mp-easy/bin/tes3mp-easy-ops --root /tes3mp-easy --json backups create --type state --if-running",
				"TimeoutStartSec=9min",
				"Environment=GOMAXPROCS=1",
				"Environment=GOMEMLIMIT=64MiB",
				"MemoryHigh=48M",
				"MemoryMax=64M",
				"OOMScoreAdjust=500",
				"Nice=19",
				"IOSchedulingClass=idle",
				"UMask=0027",
			},
		},
		{
			name: "backup timer",
			path: []string{"systemd", "tes3mp-easy-backup.timer"},
			want: []string{
				"OnBootSec=2min",
				"OnUnitActiveSec=15min",
				"AccuracySec=30s",
				"Unit=tes3mp-easy-backup.service",
				"WantedBy=timers.target",
			},
		},
		{
			name: "journald drop-in",
			path: []string{"systemd", "journald.conf.d", "tes3mp-easy.conf"},
			want: []string{
				"[Journal]",
				"SystemMaxUse=100M",
				"RuntimeMaxUse=32M",
			},
		},
		{
			name: "sysctl drop-in",
			path: []string{"sysctl.d", "99-tes3mp-easy.conf"},
			want: []string{"vm.swappiness=10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := projectPath(t, tt.path...)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			content := string(data)
			for _, want := range tt.want {
				if !strings.Contains(content, want) {
					t.Fatalf("%s does not contain %q:\n%s", filepath.Base(path), want, content)
				}
			}
		})
	}
}

func TestPanelUnitUsesOneWritableMount(t *testing.T) {
	data, err := os.ReadFile(projectPath(t, "systemd", "tes3mp-panel.service"))
	if err != nil {
		t.Fatalf("read panel unit: %v", err)
	}

	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ReadWritePaths=") {
			paths = append(paths, line)
		}
	}
	if len(paths) != 1 || paths[0] != "ReadWritePaths=/tes3mp-easy" {
		t.Fatalf("panel unit must expose exactly one writable root, got %q", paths)
	}
}

func TestServerUnitDoesNotGetPrivilegesItDoesNotNeed(t *testing.T) {
	data, err := os.ReadFile(projectPath(t, "systemd", "tes3mp.service"))
	if err != nil {
		t.Fatalf("read server unit: %v", err)
	}
	content := string(data)
	for _, forbidden := range []string{
		"CAP_NET_BIND_SERVICE",
		"SupplementaryGroups=systemd-journal",
		"NoNewPrivileges=no",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("server unit must not contain %q:\n%s", forbidden, content)
		}
	}
}
