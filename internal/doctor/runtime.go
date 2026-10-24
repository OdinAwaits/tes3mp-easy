package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type runtimeFile struct {
	rel        string
	executable bool
	dir        bool
}

var runtimeFiles = []runtimeFile{
	{rel: "tes3mp-server", executable: true},
	{rel: "tes3mp-server.x86_64", executable: true},
	{rel: "tes3mp-prelaunch", executable: true},
	{rel: "openmw.cfg"},
	{rel: "tes3mp-server-default.cfg"},
	{rel: "server/scripts/config.lua"},
	{rel: "server/scripts/customScripts.lua"},
	{rel: "server/scripts/serverCore.lua"},
	{rel: "server/data/requiredDataFiles.json"},
	{rel: "lib", dir: true},
}

func checkRuntime(ctx context.Context, root string, timeout time.Duration,
	run func(context.Context, string, ...string) ([]byte, error),
) Check {
	if root == "" {
		return errorCheck("runtime", "runtime path is empty")
	}
	if run == nil {
		return errorCheck("runtime", "command runner is not configured")
	}

	info, err := os.Stat(root)
	if err != nil {
		return errorCheck("runtime", "cannot access %s: %v", root, err)
	}
	if !info.IsDir() {
		return errorCheck("runtime", "%s is not a directory", root)
	}

	problems := make([]string, 0, len(runtimeFiles)+2)
	binariesPresent := true
	for _, required := range runtimeFiles {
		path := filepath.Join(root, required.rel)
		info, err := os.Stat(path)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", required.rel, err))
			if required.executable {
				binariesPresent = false
			}
			continue
		case required.dir && !info.IsDir():
			problems = append(problems, fmt.Sprintf("%s is not a directory", required.rel))
			continue
		case !required.dir && !info.Mode().IsRegular():
			problems = append(problems, fmt.Sprintf("%s is not a regular file", required.rel))
			continue
		}
		if required.executable && info.Mode().Perm()&0o111 == 0 {
			problems = append(problems,
				fmt.Sprintf("%s is not executable", required.rel))
			binariesPresent = false
		}
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	ldconfigOut, err := run(callCtx, "ldconfig", "-p")
	cancel()
	if err != nil {
		problems = append(problems, fmt.Sprintf("ldconfig -p: %v", err))
	} else if !strings.Contains(string(ldconfigOut), "libluajit-5.1.so.2") {
		problems = append(problems,
			"libluajit-5.1.so.2 is not registered in ldconfig")
	}

	if binariesPresent {
		target := filepath.Join(root, "tes3mp-server.x86_64")
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		// The tes3mp-server wrapper starts the binary with LD_LIBRARY_PATH=./lib.
		// Use the same path for ldd, otherwise the bundled boost libraries are
		// reported as "not found" on a healthy runtime.
		lddOut, err := run(callCtx, "env",
			"LD_LIBRARY_PATH="+filepath.Join(root, "lib"), "ldd", target)
		cancel()
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("ldd %s: %v", target, err))
		case strings.Contains(string(lddOut), "not found"):
			problems = append(problems, "ldd reports missing shared libraries")
		}
	}

	if len(problems) > 0 {
		return errorCheck("runtime", "%s", joinProblems(problems))
	}
	return okCheck("runtime", "TES3MP runtime verified")
}
