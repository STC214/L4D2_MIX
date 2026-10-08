package main

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

func validateBuiltFiles(files []builtFile, requireHash bool) error {
	if len(files) == 0 {
		return fmt.Errorf("构建清单没有产物，请重新合并")
	}
	seen := map[string]bool{}
	for _, file := range files {
		if !filepath.IsLocal(file.Name) || filepath.Base(file.Name) != file.Name || !strings.EqualFold(filepath.Ext(file.Name), ".vpk") {
			return fmt.Errorf("构建清单包含无效文件名: %q", file.Name)
		}
		key := strings.ToLower(file.Name)
		if seen[key] {
			return fmt.Errorf("构建清单包含重复文件名: %q", file.Name)
		}
		seen[key] = true
		if file.Size < 0 {
			return fmt.Errorf("构建清单包含无效文件大小: %q", file.Name)
		}
		if requireHash {
			digest, err := hex.DecodeString(file.SHA256)
			if err != nil || len(digest) != 32 {
				return fmt.Errorf("构建清单包含无效 SHA-256: %q", file.Name)
			}
		}
	}
	return nil
}
