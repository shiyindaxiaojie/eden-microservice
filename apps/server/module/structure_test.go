package module_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMonorepoModuleLayout(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	wantModules := map[string]string{
		"packages":      "eden-microservice/packages",
		"apps/registry": "eden-microservice/apps/registry",
		"apps/config":   "eden-microservice/apps/config",
		"apps/gateway":  "eden-microservice/apps/gateway",
		"apps/auth":     "eden-microservice/apps/auth",
		"apps/cluster":  "eden-microservice/apps/cluster",
		"apps/server":   "eden-microservice/apps/server",
	}

	workspace, err := os.ReadFile(filepath.Join(repoRoot, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	for dir, modulePath := range wantModules {
		manifest := filepath.Join(repoRoot, filepath.FromSlash(dir), "go.mod")
		data, err := os.ReadFile(manifest)
		if err != nil {
			t.Errorf("module %s is missing go.mod: %v", dir, err)
			continue
		}
		if !strings.Contains(string(data), "module "+modulePath) {
			t.Errorf("%s must declare module %s", manifest, modulePath)
		}
		workspacePath := "./" + filepath.ToSlash(dir)
		if !strings.Contains(string(workspace), workspacePath) {
			t.Errorf("go.work must include %s", workspacePath)
		}
	}

	for _, domain := range []string{"registry", "config", "gateway", "auth", "cluster"} {
		for _, child := range []string{"internal", "module", filepath.Join("cmd", "eden-"+domain)} {
			path := filepath.Join(repoRoot, "apps", domain, child)
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				t.Errorf("domain module directory is missing: %s", path)
			}
		}
	}
}

func TestModulesDoNotImportAnotherModulesInternalPackages(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	appsRoot := filepath.Join(repoRoot, "apps")
	err := filepath.WalkDir(appsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(appsRoot, path)
		if err != nil {
			return err
		}
		sourceModule := strings.Split(filepath.ToSlash(relativePath), "/")[0]
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(importPath, "eden-microservice/apps/") || !strings.Contains(importPath, "/internal/") {
				continue
			}
			importedModule := strings.Split(strings.TrimPrefix(importPath, "eden-microservice/apps/"), "/")[0]
			if importedModule == sourceModule {
				continue
			}
			t.Errorf("%s imports a module internal package: %s", path, importPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk apps: %v", err)
	}
}

func TestSharedPackagesDoNotDependOnApplications(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	packagesRoot := filepath.Join(repoRoot, "packages")
	err := filepath.WalkDir(packagesRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(importPath, "eden-microservice/apps/") {
				t.Errorf("%s imports an application package: %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk packages: %v", err)
	}
}
